package users

import (
	"errors"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
	"github.com/tzrikka/timpani-api/pkg/jira"
)

// BitbucketToEmail converts a Bitbucket account ID into an email address.
// This depends on the user's email address being the same in both systems.
// This function uses data caching, and API calls as a fallback.
func BitbucketToEmail(ctx workflow.Context, accountID string) (string, error) {
	if accountID == "" {
		return "", errors.New("missing Bitbucket account ID")
	}

	user, err := data.SelectUserByBitbucketID(accountID)
	if err != nil {
		logger.Error(ctx, "failed to load user by Bitbucket ID", err, slog.String("account_id", accountID))
		// Don't abort - use the Jira API as a fallback (Bitbucket API does not expose email addresses).
	}

	if user.Email != "" {
		return user.Email, nil
	}

	jiraUser, err := jira.UsersGet(ctx, accountID)
	if err != nil {
		logger.Error(ctx, "failed to retrieve Jira user info", err, slog.String("account_id", accountID))
		return "", err
	}

	email := jiraUser.Email
	if err := data.UpsertUser(email, accountID, "", "", "", ""); err != nil {
		logger.Error(ctx, "failed to save Bitbucket account ID/email mapping", err,
			slog.String("account_id", accountID), slog.String("email", email))
		// Don't return the error (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	}

	return email, nil
}

// BitbucketToSlackID converts a Bitbucket account ID into a Slack user ID.
// This depends on the user's email address being the same in both systems.
// This function returns an empty string if the account ID is not found.
func BitbucketToSlackID(ctx workflow.Context, accountID string, checkOptIn bool) string {
	user, err := data.SelectUserByBitbucketID(accountID)
	if err != nil {
		logger.Error(ctx, "failed to load user by Bitbucket ID", err, slog.String("account_id", accountID))
		return ""
	}

	if checkOptIn && user.ThrippyLink == "" {
		return ""
	}

	return user.SlackID
}

// BitbucketToSlackRef converts a Bitbucket account ID into a Slack user reference.
// This depends on the user's email address being the same in both systems.
// This function returns some kind of display name if the user is not found.
func BitbucketToSlackRef(ctx workflow.Context, accountID, displayName string) string {
	user, err := data.SelectUserByBitbucketID(accountID)
	if err != nil {
		logger.Error(ctx, "failed to load user by Bitbucket ID", err, slog.String("account_id", accountID))
		// Don't return the error (i.e. abort the calling workflow) - use the Bitbucket API as a fallback.
	}

	if user.SlackID != "" {
		return fmt.Sprintf("<@%s>", user.SlackID)
	}
	if user.RealName != "" {
		return user.RealName
	}

	if displayName != "" {
		return displayName
	}
	if accountID == "" {
		return "A bot"
	}

	apiUser, err := bitbucket.UsersGet(ctx, accountID, "")
	if err != nil {
		logger.Error(ctx, "failed to retrieve Bitbucket user info", err, slog.String("account_id", accountID))
		return accountID // Last resort: return the original Bitbucket account ID.
	}

	if apiUser.DisplayName != "" {
		if err := data.UpsertUser("", accountID, "", "", apiUser.DisplayName, ""); err != nil {
			logger.Error(ctx, "failed to save Bitbucket user display name", err,
				slog.String("account_id", accountID), slog.String("display_name", apiUser.DisplayName))
			// Don't return the error (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
		}
	}

	return apiUser.DisplayName
}

// EmailToBitbucketID retrieves a Bitbucket user's account ID based on their
// email address. This function uses data caching, and API calls as a fallback.
func EmailToBitbucketID(ctx workflow.Context, workspace, email string) (string, error) {
	if email == "" {
		return "", errors.New("empty email address")
	}

	user, err := data.SelectUserByEmail(email)
	if err != nil {
		logger.Error(ctx, "failed to load user by email", err, slog.String("email", email))
		// Don't return the error (i.e. abort the calling workflow) - use the Bitbucket API as a fallback.
	}

	if user.BitbucketID != "" {
		return user.BitbucketID, nil
	}

	users, err := jira.UsersSearchActivity(ctx, email)
	if err != nil {
		logger.Error(ctx, "failed to search Jira user by email", err, slog.String("email", email))
		return "", err
	}
	if len(users) == 0 {
		logger.Error(ctx, "Bitbucket user not found", nil, slog.String("email", email))
		return "", fmt.Errorf("bitbucket user account not found for %q", email)
	}
	if len(users) > 1 {
		logger.Warn(ctx, "multiple Bitbucket users found", slog.String("email", email), slog.Int("count", len(users)))
		return "", fmt.Errorf("multiple (%d) Bitbucket accounts found for %q", len(users), email)
	}

	id := users[0].AccountID
	if err := data.UpsertUser(email, id, "", "", "", ""); err != nil {
		logger.Error(ctx, "failed to save Bitbucket account ID/email mapping", err,
			slog.String("account_id", id), slog.String("email", email))
		// Don't return the error (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	}

	return id, nil
}
