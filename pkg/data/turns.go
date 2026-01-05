package data

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/jira"
)

// PRTurn represents the attention state for all the
// author-reviewer pairs in a specific pull request.
type PRTurn struct {
	Author    string          `json:"author"`    // Email address of the PR author.
	Reviewers map[string]bool `json:"reviewers"` // Email address -> is it their turn?

	FrozenAt string `json:"frozen_at,omitempty"`
	FrozenBy string `json:"frozen_by,omitempty"`
}

const (
	turnFileSuffix = "_turn"
)

var prTurnMutexes RWMutexMap

// InitTurns initializes the attention state of a new PR with its author's email address.
// The initial state has no reviewers; they are added when they are added to the Slack channel.
func InitTurns(ctx workflow.Context, url, author string) {
	// Happens only once per PR, so no need for mutex here.
	if err := writeTurnFile(ctx, url, &PRTurn{Author: author, Reviewers: map[string]bool{}}); err != nil {
		logger.From(ctx).Error("failed to initialize PR attention state", slog.Any("error", err), slog.String("pr_url", url))
	}
}

func DeleteTurns(ctx workflow.Context, url string) {
	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	if ctx == nil { // For unit testing.
		_ = deletePRFileActivity(context.Background(), url, turnFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, deletePRFileActivity, nil, url, turnFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete PR attention state", slog.Any("error", err), slog.String("pr_url", url))
	}
}

// AddReviewerToTurns adds a new reviewer to the attention state of a specific PR.
// This function is idempotent: if a reviewer already exists, or is the PR author,
// it does nothing. It also ignores empty or "bot" email addresses.
func AddReviewerToTurns(ctx workflow.Context, url, email string) error {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return nil
	}

	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to add reviewer",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return err
	}

	if _, found := t.Reviewers[email]; found || email == t.Author {
		return nil
	}
	t.Reviewers[email] = true

	if err := writeTurnFile(ctx, url, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to add reviewer",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return err
	}

	return nil
}

// GetCurrentTurn returns the email addresses of all the users whose turn it is to
// pay attention to a specific PR. If the PR has no assigned reviewers, this function
// returns the PR author (as a reminder for them to assign reviewers). If any assigned
// reviewer has their turn flag set to false, we add the author to the list as well.
func GetCurrentTurn(ctx workflow.Context, url string) ([]string, error) {
	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state of current turn",
			slog.Any("error", err), slog.String("pr_url", url))
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
// This is used when that reviewer approves the PR, or is unassigned from the PR.
// This function is idempotent: if the reviewer does not exist, it does nothing.
// It also ignores empty or "bot" email addresses.
func RemoveReviewerFromTurns(ctx workflow.Context, url, email string) error {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return nil
	}

	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to remove reviewer",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return err
	}

	if _, found := t.Reviewers[email]; !found {
		return nil
	}

	delete(t.Reviewers, email)

	if err := writeTurnFile(ctx, url, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to remove reviewer",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return err
	}

	return nil
}

// FreezeTurn marks the attention state of a specific PR as frozen by a specific user.
// This prevents any changes by [SwitchTurn], and only by it, until it is unfrozen.
// If the turn is already frozen, this function returns false and does nothing.
func FreezeTurn(ctx workflow.Context, url, email string) (bool, error) {
	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to freeze turn",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return false, err
	}

	if t.FrozenAt != "" {
		return false, nil
	}

	t.FrozenAt = time.Now().UTC().Format(time.RFC3339)
	t.FrozenBy = email

	if err := writeTurnFile(ctx, url, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to freeze turn",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return false, err
	}

	return true, nil
}

// UnfreezeTurn is the inverse of [FreezeTurn].
// If the turn is not frozen, this function returns false and does nothing.
func UnfreezeTurn(ctx workflow.Context, url string) (bool, error) {
	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to unfreeze turn",
			slog.Any("error", err), slog.String("pr_url", url))
		return false, err
	}

	if t.FrozenAt == "" {
		return false, nil
	}

	t.FrozenAt = ""
	t.FrozenBy = ""

	if err := writeTurnFile(ctx, url, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to unfreeze turn",
			slog.Any("error", err), slog.String("pr_url", url))
		return false, err
	}

	return true, nil
}

// SwitchTurn switches the turn of a specific user in a specific PR.
// If the user is not found, or if the turn is frozen, this function does nothing.
// If the user is the PR author, it adds all reviewers to the attention state.
// If the user is a reviewer, it adds the author to the attention state.
func SwitchTurn(ctx workflow.Context, url, email string) error {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return nil
	}

	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to switch turns",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return err
	}

	if t.FrozenAt != "" {
		return nil
	}

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

	if err := writeTurnFile(ctx, url, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to switch turns",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return err
	}

	return nil
}

// Nudge records that a specific user has been nudged about a specific PR,
// so it becomes their turn to pay attention to that PR if it wasn't already.
// It returns true if the nudge is valid (the user is in the current turn list).
func Nudge(ctx workflow.Context, url, email string) (bool, error) {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return false, nil
	}

	mu := prTurnMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	t, err := readTurnFile(ctx, url)
	if err != nil {
		logger.From(ctx).Error("failed to read PR attention state to nudge user",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return false, err
	}

	isTheirTurn, foundInTurns := t.Reviewers[email]
	if email == t.Author {
		// PR author's turn can be implicit (no reviewers) or set explicitly by a nudge.
		// For the sake of simplicity, we don't check the other implicit case here
		// (at least one of the reviewers has a false attention state).
		if len(t.Reviewers) == 0 || foundInTurns {
			return true, nil // Valid nudge, but a no-op.
		}
	} else {
		if !foundInTurns {
			return false, nil // Invalid nudge.
		}
		if isTheirTurn {
			return true, nil // Valid nudge, but a no-op.
		}
	}

	// Valid nudge that requires a state change.
	t.Reviewers[email] = true

	if err := writeTurnFile(ctx, url, t); err != nil {
		logger.From(ctx).Error("failed to write PR attention state to nudge user",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("email", email))
		return false, err
	}

	return true, nil
}

// readTurnFile expects the caller to hold the appropriate mutex.
func readTurnFile(ctx workflow.Context, url string) (*PRTurn, error) {
	path, err := cachedDataPath(url, turnFileSuffix)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return resetTurns(ctx, url)
		}
		return nil, err
	}
	defer f.Close()

	t := new(PRTurn)
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return resetTurns(ctx, url)
	}
	if t.Author == "" {
		return resetTurns(ctx, url)
	}

	normalizeEmailAddresses(t)
	return t, nil
}

func normalizeEmailAddresses(t *PRTurn) {
	t.Author = strings.ToLower(t.Author)
	t.FrozenBy = strings.ToLower(t.FrozenBy)
	for reviewer, state := range t.Reviewers {
		if strings.ToLower(reviewer) == reviewer {
			continue
		}
		t.Reviewers[strings.ToLower(reviewer)] = state
		delete(t.Reviewers, reviewer)
	}
}

// writeTurnFile wraps [writeTurnFileActivity] and expects the caller to hold the appropriate mutex.
func writeTurnFile(ctx workflow.Context, url string, t *PRTurn) error {
	if ctx == nil { // For unit testing.
		return writeTurnFileActivity(context.Background(), url, t)
	}

	return executeLocalActivity(ctx, writeTurnFileActivity, nil, url, t)
}

// writeTurnFileActivity runs as a local activity and expects the caller to hold the appropriate mutex.
func writeTurnFileActivity(_ context.Context, url string, t *PRTurn) error {
	path, err := cachedDataPath(url, turnFileSuffix)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // URL received from signature-verified 3rd-party.
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
func resetTurns(ctx workflow.Context, url string) (*PRTurn, error) {
	logger.From(ctx).Warn("resetting PR attention state file", slog.String("pr_url", url))

	snapshot, err := LoadBitbucketPR(ctx, url)
	if err != nil {
		return nil, err
	}

	author := userEmail(ctx, snapshot["author"])

	reviewers := map[string]bool{}
	jsonList, ok := snapshot["reviewers"].([]any)
	if !ok {
		jsonList = []any{}
	}
	for _, r := range jsonList {
		reviewers[userEmail(ctx, r)] = true
	}

	t := &PRTurn{Author: author, Reviewers: reviewers}
	return t, writeTurnFile(ctx, url, t)
}

// userEmail extracts the Bitbucket account ID from user details map, and converts
// it into the user's email address, based on RevChat's own user database.
func userEmail(ctx workflow.Context, detailsMap any) string {
	m, ok := detailsMap.(map[string]any)
	if !ok {
		return ""
	}

	accountID, ok := m["account_id"].(string)
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
		logger.From(ctx).Error("failed to retrieve Jira user info",
			slog.Any("error", err), slog.String("account_id", accountID))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	email := strings.ToLower(jiraUser.Email)
	_ = UpsertUser(ctx, email, "", accountID, "", "", "")

	return email
}
