package data

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data/internal"
)

const (
	// SlackIDNotFound is a key prefix used in [ListPRsPerSlackUser] and by its callers.
	// It indicates that the key suffix (email address) couldn't be matched to a Slack user ID.
	SlackIDNotFound = internal.SlackIDNotFound
)

// InitTurns initializes the attention state of a new PR with its author's email address.
// The initial state has no reviewers; they are added when they are added to the Slack channel.
// Happens only once per PR, in the beginning, so no need for a Temporal activity, mutex, etc.
func InitTurns(ctx workflow.Context, prURL, authorEmail string) {
	if err := internal.InitTurns(prURL, authorEmail); err != nil {
		logger.From(ctx).Error("failed to initialize PR attention state", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

func DeleteTurns(ctx workflow.Context, prURL string) {
	if ctx == nil { // For unit testing.
		_ = internal.DeleteGenericPRFile(context.Background(), prURL+internal.TurnsFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, internal.DeleteGenericPRFile, nil, prURL+internal.TurnsFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete PR attention state", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

// LoadCurrentTurnEmails returns the email addresses of all the users whose turn it is
// to pay attention to a specific PR. If the PR has no assigned reviewers, this function
// returns the PR author (as a reminder for them to assign reviewers). If any assigned
// reviewer has their turn flag set to false, we add the author to the list as well.
func LoadCurrentTurnEmails(ctx workflow.Context, prURL string) ([]string, error) {
	if ctx == nil { // For unit testing.
		return internal.ReadCurrentTurnEmails(context.Background(), prURL)
	}

	var emails []string
	if err := executeLocalActivity(ctx, internal.ReadCurrentTurnEmails, &emails, prURL); err != nil {
		logger.From(ctx).Warn("failed to read PR attention state", slog.Any("error", err), slog.String("pr_url", prURL))
		return nil, err
	}

	return emails, nil
}

// ListPRsPerSlackUser scans all stored PR turn files, and returns a mapping
// from Slack user IDs to all the PR URLs they need to be reminded about.
func ListPRsPerSlackUser(ctx workflow.Context, onlyCurrentTurn, authors, reviewers bool) (users map[string][]string) {
	var err error
	if ctx == nil { // For unit testing.
		users, err = internal.ReadPRsPerSlackUser(context.Background(), onlyCurrentTurn, authors, reviewers)
	} else {
		err = executeLocalActivity(ctx, internal.ReadPRsPerSlackUser, &users, onlyCurrentTurn, authors, reviewers)
	}

	if err != nil {
		logger.From(ctx).Warn("failed to read all PR attention states", slog.Any("error", err),
			slog.Bool("only_current_turn", onlyCurrentTurn), slog.Bool("authors", authors), slog.Bool("reviewers", reviewers))
		return nil
	}

	return users
}

// SetReviewerTurn records that it's a specific user's turn to review a specific PR: either
// because they were added as a reviewer, or because they're an existing reviewer and were nudged.
// This function is idempotent either way, but the return values indicate the state for nudge calls:
// The first boolean indicates whether the requested nudge is allowed (the user is tracked as a reviewer),
// and the second one indicates whether the user already approved the PR (in case the first value is false).
func SetReviewerTurn(ctx workflow.Context, prURL, email string, nudge bool) (done, approved bool, err error) {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return false, false, nil
	}

	if ctx == nil { // For unit testing.
		state, err := internal.SetReviewerTurn(context.Background(), prURL, email, nudge)
		return state[0], state[1], err
	}

	var state []bool
	if err := executeLocalActivity(ctx, internal.SetReviewerTurn, &state, prURL, email, nudge); err != nil {
		logger.From(ctx).Error("failed to set reviewer in PR attention state", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("email", email))
		return false, false, err
	}

	return state[0], state[1], nil
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

	if ctx == nil { // For unit testing.
		return internal.SwitchTurn(context.Background(), prURL, email, force)
	}

	if err := executeLocalActivity(ctx, internal.SwitchTurn, nil, prURL, email, force); err != nil {
		logger.From(ctx).Error("failed to switch turn in PR attention state", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("email", email))
		return err
	}

	return nil
}

// RemoveReviewerFromTurns completely removes a reviewer from the attention state of a specific PR.
// This is called when that reviewer approves the PR, or is unassigned from it. This function is idempotent:
// if the reviewer does not exist, it does nothing. It also ignores empty or "bot" email addresses.
func RemoveReviewerFromTurns(ctx workflow.Context, prURL, email string, approved bool) error {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return nil
	}

	if ctx == nil { // For unit testing.
		return internal.RemoveReviewerFromTurns(context.Background(), prURL, email, approved)
	}

	if err := executeLocalActivity(ctx, internal.RemoveReviewerFromTurns, nil, prURL, email, approved); err != nil {
		logger.From(ctx).Error("failed to remove reviewer from PR attention state", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("email", email))
		return err
	}

	return nil
}

// GetActivityTime returns the last activity timestamp of a specific user in a specific PR.
// If the user is not found or is a bot, this function returns a zero timestamp.
func GetActivityTime(ctx workflow.Context, prURL, email string) time.Time {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return time.Time{}
	}

	if ctx == nil { // For unit testing.
		t, _ := internal.GetActivityTime(context.Background(), prURL, email)
		return t
	}

	var t time.Time
	if err := executeLocalActivity(ctx, internal.GetActivityTime, &t, prURL, email); err != nil {
		logger.From(ctx).Error("failed to get PR reviewer's activity timestamp", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("email", email))
		return time.Time{}
	}

	return t
}

// UpdateActivityTime updates the last activity timestamp of a specific user in a
// specific PR. If the user is not found or is a bot, this function does nothing.
// This is called when the user interacts with the PR in any way that doesn't
// change their turn (such as PR edits, commit pushes, and review actions).
func UpdateActivityTime(ctx workflow.Context, prURL, email string) {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return
	}

	if ctx == nil { // For unit testing.
		_ = internal.UpdateActivityTime(context.Background(), prURL, email)
		return
	}

	if err := executeLocalActivity(ctx, internal.UpdateActivityTime, nil, prURL, email); err != nil {
		logger.From(ctx).Error("failed to update PR reviewer's activity timestamp", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("email", email))
	}
}

// FreezeTurns marks the attention state of a specific PR as frozen by a specific user.
// This prevents most changes by [SwitchTurn], and only by it, until it is unfrozen.
// If the turn is already frozen, this function returns false and does nothing.
func FreezeTurns(ctx workflow.Context, prURL, email string) (bool, error) {
	email = strings.ToLower(email)
	if email == "" || email == "bot" {
		return false, nil
	}

	if ctx == nil { // For unit testing.
		return internal.FreezeTurns(context.Background(), prURL, email)
	}

	if err := executeLocalActivity(ctx, internal.FreezeTurns, nil, prURL, email); err != nil {
		logger.From(ctx).Error("failed to freeze PR attention state", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("email", email))
		return false, err
	}

	return true, nil
}

// UnfreezeTurns is the inverse of [FreezeTurns].
// If the turn is not frozen, this function returns false and does nothing.
func UnfreezeTurns(ctx workflow.Context, prURL string) (bool, error) {
	if ctx == nil { // For unit testing.
		return internal.UnfreezeTurns(context.Background(), prURL)
	}

	if err := executeLocalActivity(ctx, internal.UnfreezeTurns, nil, prURL); err != nil {
		logger.From(ctx).Error("failed to unfreeze PR attention state", slog.Any("error", err),
			slog.String("pr_url", prURL))
		return false, err
	}

	return true, nil
}

// IsFrozen returns the timestamp and user email of when and who froze the attention state of
// a specific PR. If the turn is not frozen, it returns a zero timestamp and an empty string.
func IsFrozen(ctx workflow.Context, prURL string) (time.Time, string) {
	if ctx == nil { // For unit testing.
		frozen, _ := internal.IsFrozen(context.Background(), prURL)
		return frozen.At, frozen.By
	}

	var frozen internal.Frozen
	if err := executeLocalActivity(ctx, internal.IsFrozen, &frozen, prURL); err != nil {
		logger.From(ctx).Error("failed to get PR attention state", slog.Any("error", err),
			slog.String("pr_url", prURL))
		return time.Time{}, ""
	}

	return frozen.At, frozen.By
}
