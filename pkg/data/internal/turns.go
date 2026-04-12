package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/timpani-api/pkg/jira"
	"github.com/tzrikka/timpani-api/pkg/slack"
	"github.com/tzrikka/xdg"
)

const (
	TurnsFileSuffix = "_turns.json"
	SlackIDNotFound = " email not found"
)

// PRTurns represents the attention state for all the author-reviewer
// pairs in a specific pull request, and also records activity timestamps.
type PRTurns struct {
	Author string `json:"author"` // Email address of the PR author.

	Reviewers map[string]bool      `json:"reviewers,omitempty"` // Email address -> is it their turn?
	Activity  map[string]time.Time `json:"activity,omitempty"`  // When each user last interacted with the PR.
	Approvers map[string]time.Time `json:"approvers,omitempty"` // When each user approved the PR.

	FrozenAt time.Time `json:"frozen_at,omitzero"`
	FrozenBy string    `json:"frozen_by,omitempty"`
}

// Frozen is used to return the result of [IsFrozen] in a single struct, instead of two separate values.
type Frozen struct {
	At time.Time `json:"at"`
	By string    `json:"by"`
}

// InitTurns initializes the attention state of a new PR with its author's email address.
// The initial state has no reviewers; they are added when they are added to the Slack channel.
// Happens only once per PR, in the beginning, so no need for a Temporal activity, mutex, etc.
func InitTurns(prURL, authorEmail string) error {
	return writeTurns(prURL, &PRTurns{Author: authorEmail})
}

// SetReviewerTurn records that it's a specific user's turn to review a specific PR: either
// because they were added as a reviewer, or because they're an existing reviewer and were nudged.
// This function is idempotent either way, but the return values indicate the state for nudge calls:
// The first boolean indicates whether the requested nudge is allowed (the user is tracked as a reviewer),
// and the second one indicates whether the user already approved the PR (in case the first value is false).
func SetReviewerTurn(ctx context.Context, opts client.Options, prURL, email string, nudge bool) ([2]bool, error) {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return [2]bool{false, false}, err
	}

	alreadyTheirTurn, foundInTurns := t.Reviewers[email]
	// Add new reviewer?
	if !nudge {
		// Yes, but a no-op (PR author, or already their turn).
		if email == t.Author || foundInTurns {
			return [2]bool{true, false}, nil
		}
	}
	// Or nudge an existing reviewer?
	if nudge {
		if email == t.Author {
			// Yes, but a no-op: PR author's turn can be implicit (no reviewers) or set explicitly by forced turn switch.
			if len(t.Reviewers) == 0 || foundInTurns {
				return [2]bool{true, false}, nil
			}
		} else {
			// No, only tracked reviewers may be nudged (but maybe they already approved the PR?).
			if !foundInTurns {
				return [2]bool{false, !t.Approvers[email].IsZero()}, nil
			}
			// Yes, but a no-op: it's already their turn.
			if alreadyTheirTurn {
				return [2]bool{true, false}, nil
			}
		}
	}
	// Valid and necessary state change.
	t.Reviewers[email] = true

	if err := writeTurns(prURL, t); err != nil {
		return [2]bool{false, false}, err
	}

	return [2]bool{true, false}, nil
}

// SwitchTurn switches the turn of a specific user in a specific PR to others.
// If the user is not found or is a bot, this function does nothing.
// If turns are frozen and the switch isn't forced, it only records the activity.
// If the user is the PR author, it adds all reviewers to the attention state.
// If the user is a reviewer, it adds the author to the attention state.
func SwitchTurn(ctx context.Context, opts client.Options, prURL, email string, force bool) error {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return err
	}

	if t.FrozenAt.IsZero() || force {
		if email == t.Author {
			delete(t.Reviewers, email) // In case the author was added via [Nudge].
			for reviewer := range t.Reviewers {
				t.Reviewers[reviewer] = true
			}
		} else {
			if _, found := t.Reviewers[email]; found {
				t.Reviewers[email] = false
			}
		}
	}

	t.Activity[email] = time.Now().UTC() // Record activity regardless of frozen state.

	if err := writeTurns(prURL, t); err != nil {
		return err
	}

	return nil
}

// RemoveReviewerFromTurns completely removes a reviewer from the attention state of a specific PR. This is called when that
// reviewer approves the PR, or is unassigned from it. This function is idempotent: if the reviewer does not exist, it does nothing.
func RemoveReviewerFromTurns(ctx context.Context, opts client.Options, prURL, email string, approved bool) error {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return err
	}

	if _, found := t.Reviewers[email]; !found {
		return nil
	}

	delete(t.Reviewers, email)
	now := time.Now().UTC()
	t.Activity[email] = now
	if approved {
		t.Approvers[email] = now
	}

	if err := writeTurns(prURL, t); err != nil {
		return err
	}

	return nil
}

// ReadCurrentTurnEmails returns the email addresses of all the users whose turn it is
// to pay attention to a specific PR. If the PR has no assigned reviewers, this function
// returns the PR author (as a reminder for them to assign reviewers). If any assigned
// reviewer has their turn flag set to false, we add the author to the list as well.
func ReadCurrentTurnEmails(ctx context.Context, opts client.Options, prURL string) ([]string, error) {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return nil, err
	}

	if len(t.Reviewers) == 0 && t.Author != "" && t.Author != "bot" {
		return []string{t.Author}, nil
	}

	emails := make([]string, 0, len(t.Reviewers)+1)
	includeAuthor := false
	for email, isTurn := range t.Reviewers {
		if isTurn {
			emails = append(emails, email)
		} else {
			includeAuthor = true
		}
	}
	if includeAuthor && t.Author != "" && t.Author != "bot" {
		emails = append(emails, t.Author)
	}

	slices.Sort(emails)
	return slices.Compact(emails), nil
}

// readAllParticipantEmails is similar to [ReadCurrentTurnEmails] but returns the email addresses of all
// users that are currently tracked in the attention state of a specific PR, regardless of whether it's their
// turn or not. This includes the author and all the reviewers, including those who already approved the PR.
func readAllParticipantEmails(ctx context.Context, opts client.Options, prURL string, authors, reviewers bool) ([]string, error) {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return nil, err
	}

	var emails []string
	if authors && t.Author != "" && t.Author != "bot" {
		emails = append(emails, t.Author)
	}
	if reviewers {
		emails = slices.AppendSeq(slices.AppendSeq(emails, maps.Keys(t.Reviewers)), maps.Keys(t.Approvers))
	}

	slices.Sort(emails)
	return slices.Compact(emails), nil
}

// ReadPRsPerSlackUser scans all stored PR turn files, and returns a mapping
// from Slack user IDs to all the PR URLs they need to be reminded about.
func ReadPRsPerSlackUser(ctx context.Context, op client.Options, currentTurn, authors, reviewers bool, filter []string) (map[string][]string, error) {
	root, err := xdg.CreateDir(xdg.DataHome, config.DirName)
	if err != nil {
		return nil, err
	}

	users := map[string][]string{}
	err = fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), TurnsFileSuffix) {
			return nil
		}

		prURL := "https://" + strings.TrimSuffix(path, TurnsFileSuffix)
		var emails []string
		if currentTurn {
			emails, err = ReadCurrentTurnEmails(ctx, op, prURL)
		} else {
			emails, err = readAllParticipantEmails(ctx, op, prURL, authors, reviewers)
		}
		if err != nil {
			return nil // Skip files with errors, but keep scanning the rest.
		}

		for _, email := range emails {
			if email == "" || email == "bot" {
				continue // Ignore missing/bot users - no Slack ID to look-up.
			}

			// Valid but unrecognized (and specifically not opted-in) emails - remove from turns.
			// Example: user deactivated after being added to the PR.
			id := emailToSlackID(ctx, op, email)
			if id == "" {
				id = email + SlackIDNotFound
				users[id] = append(users[id], prURL)
				_ = RemoveReviewerFromTurns(ctx, op, prURL, email, false)
				continue
			}

			users[id] = append(users[id], prURL)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(filter) == 0 {
		return users, nil
	}

	filteredUserPRs := make(map[string][]string, len(filter))
	for _, userID := range filter {
		if prs, found := users[userID]; found || strings.HasSuffix(userID, SlackIDNotFound) {
			filteredUserPRs[userID] = prs
		}
	}

	return filteredUserPRs, nil
}

// GetActivityTime returns the last activity timestamp of a specific user in a specific PR.
// If the user is not found or is a bot, this function returns a zero timestamp.
func GetActivityTime(ctx context.Context, opts client.Options, prURL, email string) (time.Time, error) {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return time.Time{}, err
	}

	return t.Activity[email], nil
}

// UpdateActivityTime updates the last activity timestamp of a specific user in a
// specific PR. If the user is not found or is a bot, this function does nothing.
// This is called when the user interacts with the PR in any way that doesn't
// change their turn (such as PR edits, commit pushes, and review actions).
func UpdateActivityTime(ctx context.Context, opts client.Options, prURL, email string) error {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return err
	}

	t.Activity[email] = time.Now().UTC()

	if err := writeTurns(prURL, t); err != nil {
		return err
	}

	return nil
}

// FreezeTurns marks the attention state of a specific PR as frozen by a specific user.
// This prevents most changes by [SwitchTurn], and only by it, until it is unfrozen.
// If the turn is already frozen, this function returns false and does nothing.
func FreezeTurns(ctx context.Context, opts client.Options, prURL, email string) (bool, error) {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return false, err
	}

	if !t.FrozenAt.IsZero() {
		return false, nil
	}

	t.FrozenAt = time.Now().UTC()
	t.FrozenBy = email

	if err := writeTurns(prURL, t); err != nil {
		return false, err
	}

	return true, nil
}

// UnfreezeTurns is the inverse of [FreezeTurns].
// If the turn is not frozen, this function returns false and does nothing.
func UnfreezeTurns(ctx context.Context, opts client.Options, prURL string) (bool, error) {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return false, err
	}

	if t.FrozenAt.IsZero() {
		return false, nil
	}

	t.FrozenAt = time.Time{}
	t.FrozenBy = ""

	if err := writeTurns(prURL, t); err != nil {
		return false, err
	}

	return true, nil
}

// IsFrozen returns the timestamp and user email of when and who froze the attention state of
// a specific PR. If the turn is not frozen, it returns a zero timestamp and an empty string.
func IsFrozen(ctx context.Context, opts client.Options, prURL string) (Frozen, error) {
	mu := getDataFileMutex(prURL + TurnsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurns(ctx, opts, prURL)
	if err != nil {
		return Frozen{}, err
	}

	return Frozen{At: t.FrozenAt, By: t.FrozenBy}, nil
}

// writeTurns expects the calling function to hold the appropriate mutex for the given PR URL.
func writeTurns(prURL string, t *PRTurns) error {
	normalizeEmailAddresses(t)
	return writeGenericJSONFile(prURL+TurnsFileSuffix, t)
}

// normalizeEmailAddresses ensures that all email addresses are lowercase, just in case.
func normalizeEmailAddresses(t *PRTurns) {
	t.Author = strings.ToLower(t.Author)
	t.FrozenBy = strings.ToLower(t.FrozenBy)

	for user, state := range t.Reviewers {
		if strings.ToLower(user) == user {
			continue
		}
		t.Reviewers[strings.ToLower(user)] = state
		delete(t.Reviewers, user)
	}

	for _, m := range []map[string]time.Time{t.Activity, t.Approvers} {
		for user, timestamp := range m {
			if strings.ToLower(user) == user {
				continue
			}
			if !timestamp.IsZero() {
				m[strings.ToLower(user)] = timestamp
			}
			delete(m, user)
		}
	}
}

// readTurns expects the calling function to hold the appropriate mutex for the given PR URL.
func readTurns(ctx context.Context, opts client.Options, prURL string) (*PRTurns, error) {
	path, err := dataPath(prURL + TurnsFileSuffix)
	if err != nil {
		return nil, fmt.Errorf("failed to get file path: %w", err)
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resetTurns(ctx, opts, prURL, nil)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	t := new(PRTurns)
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return resetTurns(ctx, opts, prURL, nil)
	}

	// Data sanity checks.
	if t.Author == "" {
		return resetTurns(ctx, opts, prURL, t)
	}
	normalizeEmailAddresses(t)
	if t.Reviewers == nil {
		t.Reviewers = make(map[string]bool)
	}
	if t.Activity == nil {
		t.Activity = make(map[string]time.Time)
	}
	if t.Approvers == nil {
		t.Approvers = make(map[string]time.Time)
	}

	return t, nil
}

// resetTurns recreates the attention state file for a specific PR, based on the current
// PR snapshot. This is a fallback for when the turn file is missing or corrupted.
func resetTurns(ctx context.Context, opts client.Options, prURL string, t *PRTurns) (*PRTurns, error) {
	pr, err := ReadPRSnapshot(ctx, prURL)
	if err != nil {
		return nil, fmt.Errorf("failed to reset turns file due to PR snapshot error: %w", err)
	}
	if pr == nil {
		return nil, errors.New("failed to reset turns file due to missing PR snapshot")
	}

	author, accountType := userEmailAndType(ctx, opts, pr["author"])
	if author == "" && accountType == "app_user" {
		author = "bot"
	}

	// If the only thing that needs fixing is the author's email, do just that.
	if t != nil && len(t.Reviewers)+len(t.Activity)+len(t.Approvers) > 0 {
		t.Author = author
		if t.Reviewers == nil {
			t.Reviewers = make(map[string]bool)
		}
		if t.Activity == nil {
			t.Activity = make(map[string]time.Time)
		}
		if t.Approvers == nil {
			t.Approvers = make(map[string]time.Time)
		}

		if err := writeTurns(prURL, t); err != nil {
			return nil, fmt.Errorf("failed to reset turns file: %w", err)
		}
		return t, nil
	}

	// Otherwise, recreate the entire struct and file.
	reviewers := map[string]bool{}
	jsonList := listOf(pr, "reviewers")
	for _, reviewer := range jsonList {
		if email, _ := userEmailAndType(ctx, opts, reviewer); email != "" {
			reviewers[email] = true
		}
	}

	activity, approvers := map[string]time.Time{}, map[string]time.Time{}
	jsonList = listOf(pr, "participants")
	for _, participant := range jsonList {
		if email, approved, timestamp := userActivity(ctx, opts, participant); email != "" {
			activity[email] = timestamp
			if approved {
				approvers[email] = timestamp
			}
		}
	}

	t = &PRTurns{Author: author, Reviewers: reviewers, Activity: activity, Approvers: approvers}
	if err := writeTurns(prURL, t); err != nil {
		return nil, fmt.Errorf("failed to reset turns file: %w", err)
	}
	return t, nil
}

// listOf converts a given field in a PR's snapshot into a slice.
func listOf(pr map[string]any, key string) []any {
	jsonList, ok := pr[key].([]any)
	if !ok {
		return []any{}
	}
	return jsonList
}

// userActivity extracts a user's email, approval status, and activity
// time from a JSON block of participant details in a Bitbucket PR snapshot.
func userActivity(ctx context.Context, opts client.Options, detailsMap any) (string, bool, time.Time) {
	participant, ok := detailsMap.(map[string]any)
	if !ok {
		return "", false, time.Time{}
	}

	email, _ := userEmailAndType(ctx, opts, participant)
	approved, ok := participant["approved"].(bool)
	if !ok {
		approved = false
	}

	t, ok := participant["participated_on"].(string)
	if !ok {
		return email, approved, time.Time{}
	}
	parsedTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return email, approved, time.Time{}
	}

	return email, approved, parsedTime
}

// userEmailAndType extracts the Bitbucket account ID from user details map, and converts
// it into the user's email address and account type, based on RevChat's own user database.
func userEmailAndType(ctx context.Context, opts client.Options, detailsMap any) (email, accountType string) {
	user, ok := detailsMap.(map[string]any)
	if !ok {
		return "", ""
	}

	accountID, ok := user["account_id"].(string)
	if !ok {
		return "", ""
	}
	accountType, ok = user["type"].(string)
	if !ok {
		accountType = ""
	}

	return bitbucketIDToEmail(ctx, opts, accountID, accountType), accountType
}

// bitbucketIDToEmail converts a Bitbucket account ID into an email address. This function returns an empty string if the account ID
// is not found. It uses persistent data storage, or API calls as a fallback. Compare this function with [users.BitbucketIDToEmail],
// which receives a [workflow.Context] instead of a [context.Context], and returns "bot" instead of "" for non-user app accounts.
func bitbucketIDToEmail(ctx context.Context, opts client.Options, accountID, accountType string) string {
	if accountID == "" {
		return "" // Unknown user, so no point to look-up in Jira.
	}

	if user, _ := SelectUser(ctx, IndexByBitbucketID, accountID); user.Email != "" {
		return user.Email // No need to check for errors here, it's a prerequisite for the result to be non-empty.
	}

	if accountType == "app_user" || opts.Logger == nil {
		return "" // Not a real user, or a unit test, so no point to look-up in Jira.
	}

	// We use the Jira API as a fallback because the Bitbucket API doesn't expose email addresses.
	req := jira.UsersGetRequest{AccountID: accountID}
	user := new(jira.User)
	if err := executeTimpaniActivity(ctx, opts, jira.UsersGetActivityName, accountID, req, user); err != nil {
		activity.GetLogger(ctx).Warn("failed to retrieve Jira user info",
			slog.Any("error", err), slog.String("account_id", accountID))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	email := strings.ToLower(user.Email)
	_, _ = UpsertUser(ctx, email, "", accountID, "", "", "")

	return email
}

// emailToSlackID retrieves a Slack user's ID based on their email address. This function returns an
// empty string if the user ID is not found. It uses persistent data storage, or API calls as a fallback.
// Compare this function with [users.EmailToSlackID], which receives a [workflow.Context] instead of a [context.Context].
func emailToSlackID(ctx context.Context, opts client.Options, email string) string {
	if email == "" || email == "bot" {
		return ""
	}

	if user, _ := SelectUser(ctx, IndexByEmail, email); user.SlackID != "" {
		return user.SlackID // No need to check for errors here, it's a prerequisite for the result to be non-empty.
	}

	if opts.Logger == nil {
		return "" // Unit test, so no point to look-up in Slack.
	}

	req := slack.UsersLookupByEmailRequest{Email: email}
	user := new(slack.User)
	if err := executeTimpaniActivity(ctx, opts, slack.UsersLookupByEmailActivityName, email, req, user); err != nil {
		activity.GetLogger(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("email", email))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	_, _ = UpsertUser(ctx, email, user.RealName, "", "", user.ID, "")

	return user.ID
}
