package data

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/jira"
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

const (
	TurnsFileSuffix = "_turns.json"
)

var prTurnsMutexes RWMutexMap

// InitTurns initializes the attention state of a new PR with its author's email address.
// The initial state has no reviewers; they are added when they are added to the Slack channel.
func InitTurns(ctx workflow.Context, prURL, authorEmail string) {
	// Happens only once per PR, so no need for mutex here.
	if err := writeTurnsFile(ctx, prURL, &PRTurns{Author: authorEmail}); err != nil {
		logger.From(ctx).Error("failed to initialize PR attention state", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

func DeleteTurns(ctx workflow.Context, prURL string) {
	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	if ctx == nil { // For unit testing.
		_ = deletePRFileActivity(context.Background(), prURL+TurnsFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, deletePRFileActivity, nil, prURL+TurnsFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete PR attention state", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

// AddReviewerToTurns adds a new reviewer to the attention state of a specific PR.
// This function is idempotent: if a reviewer already exists, or is the PR author,
// it does nothing. It also ignores empty or "bot" email addresses.
func AddReviewerToTurns(ctx workflow.Context, prURL, email string) error {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return nil
	}

	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to add reviewer",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return err
	}

	if _, found := t.Reviewers[email]; found || email == t.Author {
		return nil
	}
	t.Reviewers[email] = true

	if err := writeTurnsFile(ctx, prURL, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to add reviewer",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return err
	}

	return nil
}

// GetCurrentTurn returns the email addresses of all the users whose turn it is to
// pay attention to a specific PR. If the PR has no assigned reviewers, this function
// returns the PR author (as a reminder for them to assign reviewers). If any assigned
// reviewer has their turn flag set to false, we add the author to the list as well.
func GetCurrentTurn(ctx workflow.Context, prURL string) ([]string, error) {
	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state of current turn",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return nil, err
	}

	if len(t.Reviewers) == 0 {
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
	if includeAuthor {
		emails = append(emails, t.Author)
	}

	slices.Sort(emails)
	return slices.Compact(emails), nil
}

// RemoveReviewerFromTurns completely removes a reviewer from the attention state of a specific PR.
// This is called when that reviewer approves the PR, or is unassigned from it. This function is idempotent:
// if the reviewer does not exist, it does nothing. It also ignores empty or "bot" email addresses.
func RemoveReviewerFromTurns(ctx workflow.Context, prURL, email string, approved bool) error {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return nil
	}

	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to remove reviewer",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return err
	}

	if _, found := t.Reviewers[email]; !found {
		return nil
	}

	delete(t.Reviewers, email)
	t.Activity[email] = now(ctx)
	if approved {
		t.Approvers[email] = now(ctx)
	}

	if err := writeTurnsFile(ctx, prURL, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to remove reviewer",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return err
	}

	return nil
}

// FreezeTurns marks the attention state of a specific PR as frozen by a specific user.
// This prevents most changes by [SwitchTurn], and only by it, until it is unfrozen.
// If the turn is already frozen, this function returns false and does nothing.
func FreezeTurns(ctx workflow.Context, prURL, email string) (bool, error) {
	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to freeze turn",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return false, err
	}

	if !t.FrozenAt.IsZero() {
		return false, nil
	}

	t.FrozenAt = now(ctx)
	t.FrozenBy = email

	if err := writeTurnsFile(ctx, prURL, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to freeze turn",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return false, err
	}

	return true, nil
}

// UnfreezeTurns is the inverse of [FreezeTurns].
// If the turn is not frozen, this function returns false and does nothing.
func UnfreezeTurns(ctx workflow.Context, prURL string) (bool, error) {
	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to unfreeze turn",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return false, err
	}

	if t.FrozenAt.IsZero() {
		return false, nil
	}

	t.FrozenAt = time.Time{}
	t.FrozenBy = ""

	if err := writeTurnsFile(ctx, prURL, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to unfreeze turn",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return false, err
	}

	return true, nil
}

func Frozen(ctx workflow.Context, url string) (time.Time, string) {
	mu := prTurnsMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to check frozen state",
			slog.Any("error", err), slog.String("pr_url", url))
		return time.Time{}, ""
	}

	return t.FrozenAt, t.FrozenBy
}

// UpdateActivityTime updates the last activity timestamp of a specific user
// in a specific PR. If the user is empty or "bot", this function does nothing.
// This is called when the user interacts with the PR in any way that doesn't
// change their turn (such as PR edits, commit pushes, and review actions).
func UpdateActivityTime(ctx workflow.Context, prURL, email string) {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return
	}

	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to update activity",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return
	}

	t.Activity[email] = now(ctx)
	if err := writeTurnsFile(ctx, prURL, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to update activity",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
	}
}

// SwitchTurn switches the turn of a specific user in a specific PR to others.
// If the user is not found or is a bot, this function does nothing.
// If turns are frozen and the switch isn't forced, it only records the activity.
// If the user is the PR author, it adds all reviewers to the attention state.
// If the user is a reviewer, it adds the author to the attention state.
func SwitchTurn(ctx workflow.Context, prURL, email string, force bool) error {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return nil
	}

	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to switch turns",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
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

	t.Activity[email] = now(ctx) // Record activity regardless of frozen state.

	if err := writeTurnsFile(ctx, prURL, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to switch turns",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return err
	}

	return nil
}

// Nudge records that a specific user has been nudged about a specific PR,
// so it becomes their turn to pay attention to that PR if it wasn't already. The first
// boolean return value indicates whether the nudge is valid (the user is tracked as a reviewer).
// The second indicates if the user already approved the PR (in case the first value is false).
func Nudge(ctx workflow.Context, prURL, email string) (ok, approved bool, err error) {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return false, false, nil
	}

	mu := prTurnsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnsFile(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to nudge user",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return false, false, err
	}

	isTheirTurn, foundInTurns := t.Reviewers[email]
	if email == t.Author {
		// PR author's turn can be implicit (no reviewers) or set explicitly by a nudge.
		// For the sake of simplicity, we don't check the other implicit case here
		// (i.e. at least one of the reviewers has a false attention state).
		if len(t.Reviewers) == 0 || foundInTurns {
			return true, false, nil // Valid nudge, but a no-op.
		}
	} else {
		if !foundInTurns {
			return false, !t.Approvers[email].IsZero(), nil // Invalid nudge.
		}
		if isTheirTurn {
			return true, false, nil // Valid nudge, but a no-op.
		}
	}

	// Valid nudge that requires a state change.
	t.Reviewers[email] = true

	if err := writeTurnsFile(ctx, prURL, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to nudge user",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
		return false, false, err
	}

	return true, false, nil
}

// readTurnsFile expects the caller to hold the appropriate mutex.
func readTurnsFile(ctx workflow.Context, url string) (*PRTurns, error) {
	path, err := cachedDataPath(url + TurnsFileSuffix)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resetTurns(ctx, url)
		}
		return nil, err
	}
	defer f.Close()

	t := new(PRTurns)
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return resetTurns(ctx, url)
	}

	// Data sanity checks.
	if t.Author == "" {
		return resetTurns(ctx, url)
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

// writeTurnsFile wraps [writeTurnFileActivity] and expects the caller to hold the appropriate mutex.
func writeTurnsFile(ctx workflow.Context, url string, t *PRTurns) error {
	if ctx == nil { // For unit testing.
		return writeTurnsFileActivity(context.Background(), url, t)
	}

	return executeLocalActivity(ctx, writeTurnsFileActivity, nil, url, t)
}

// writeTurnsFileActivity runs as a local activity and expects the caller to hold the appropriate mutex.
func writeTurnsFileActivity(_ context.Context, url string, t *PRTurns) error {
	path, err := cachedDataPath(url + TurnsFileSuffix)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		return err
	}
	defer f.Close()

	normalizeEmailAddresses(t)

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(t)
}

// resetTurns recreates the attention state file for a specific PR, based on the current
// PR snapshot. This is a fallback for when the turn file is missing or corrupted.
func resetTurns(ctx workflow.Context, url string) (*PRTurns, error) {
	logger.From(ctx).Warn("resetting PR attention state file", slog.String("pr_url", url))

	pr, err := LoadBitbucketPR(ctx, url)
	if err != nil {
		return nil, err
	}

	author := userEmail(ctx, pr["author"])

	reviewers := map[string]bool{}
	jsonList, ok := pr["reviewers"].([]any)
	if !ok {
		jsonList = []any{}
	}
	for _, r := range jsonList {
		if email := userEmail(ctx, r); email != "" {
			reviewers[email] = true
		}
	}

	t := &PRTurns{Author: author, Reviewers: reviewers, Activity: map[string]time.Time{}, Approvers: map[string]time.Time{}}
	return t, writeTurnsFile(ctx, url, t)
}

// userEmail extracts the Bitbucket account ID from user details map, and converts
// it into the user's email address, based on RevChat's own user database.
func userEmail(ctx workflow.Context, detailsMap any) string {
	user, ok := detailsMap.(map[string]any)
	if !ok {
		return ""
	}

	accountID, ok := user["account_id"].(string)
	if !ok {
		return ""
	}

	return BitbucketIDToEmail(ctx, accountID)
}

// BitbucketIDToEmail converts a Bitbucket account ID into an email address. This function returns an empty
// string if the account ID is not found. It uses persistent data storage, or API calls as a fallback.
// This function is also wrapped in the "users" package, and reused by other packages from there.
func BitbucketIDToEmail(ctx workflow.Context, accountID string) string {
	if accountID == "" {
		return ""
	}

	if user := SelectUserByBitbucketID(ctx, accountID); user.Email != "" {
		return user.Email
	}

	// We use the Jira API as a fallback because the Bitbucket API doesn't expose email addresses.
	jiraUser, err := jira.UsersGet(ctx, accountID)
	if err != nil {
		logger.From(ctx).Warn("failed to retrieve Jira user info",
			slog.Any("error", err), slog.String("account_id", accountID))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	email := strings.ToLower(jiraUser.Email)
	_ = UpsertUser(ctx, email, "", accountID, "", "", "")

	return email
}

// now returns the current time in UTC, using [workflow.Now] if possible, or [time.Now] in unit testing.
func now(ctx workflow.Context) time.Time {
	if ctx != nil {
		return workflow.Now(ctx).UTC()
	}
	return time.Now().UTC() // For unit testing.
}
