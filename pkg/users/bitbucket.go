package users

import (
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
	"github.com/tzrikka/timpani-api/pkg/jira"
)

// EmailToBitbucketID retrieves a Bitbucket user's account ID based on their email address. This function returns
// an empty string if the account ID is not found. It uses persistent data storage, or API calls as a fallback.
func EmailToBitbucketID(ctx workflow.Context, workspace, email string) string {
	if email == "" {
		return ""
	}

	if user := data.SelectUserByEmail(ctx, email); user.BitbucketID != "" {
		return user.BitbucketID
	}

	// We use the Jira API as a fallback because the Bitbucket API doesn't expose email addresses.
	users, err := jira.UsersSearchActivity(ctx, strings.ToLower(email))
	if err != nil {
		logger.From(ctx).Error("failed to search Jira user by email", slog.Any("error", err), slog.String("email", email))
		return ""
	}
	if len(users) == 0 {
		logger.From(ctx).Error("Bitbucket user not found", slog.String("email", email))
		return ""
	}
	if len(users) > 1 {
		logger.From(ctx).Warn("multiple Bitbucket users found", slog.String("email", email), slog.Int("count", len(users)))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	_ = data.UpsertUser(ctx, email, "", users[0].AccountID, "", "", "")

	return users[0].AccountID
}

// BitbucketIDToEmail is a trivial wrapper around [data.BitbucketIDToEmail].
func BitbucketIDToEmail(ctx workflow.Context, accountID string) string {
	return data.BitbucketIDToEmail(ctx, accountID)
}

// BitbucketIDToSlackID converts a Bitbucket account ID into a Slack user ID. This function returns an empty
// string if the account ID is not found. It uses persistent data storage, or API calls as a fallback.
func BitbucketIDToSlackID(ctx workflow.Context, accountID string, checkOptIn bool) string {
	user := data.SelectUserByBitbucketID(ctx, accountID)
	if user.SlackID == "" {
		// Workaround in case the user's Bitbucket account ID isn't stored, but the rest is.
		user = data.SelectUserByEmail(ctx, BitbucketIDToEmail(ctx, accountID))
	}

	if checkOptIn && user.ThrippyLink == "" {
		return ""
	}

	return user.SlackID
}

// BitbucketIDToSlackRef converts a Bitbucket account ID into a Slack user mention. This function returns a
// display name if the account ID is not found. It uses persistent data storage, or API calls as a fallback.
func BitbucketIDToSlackRef(ctx workflow.Context, accountID, displayName string) string {
	user := data.SelectUserByBitbucketID(ctx, accountID)
	if user.SlackID == "" {
		// Workaround in case the user's Bitbucket account ID isn't stored, but the rest is.
		user = data.SelectUserByEmail(ctx, BitbucketIDToEmail(ctx, accountID))
	}

	if user.SlackID != "" {
		return fmt.Sprintf("<@%s>", user.SlackID)
	}

	// Fallback 1: already-known display name.
	if user.RealName != "" {
		return user.RealName
	}
	if displayName != "" {
		return displayName
	}
	if accountID == "" {
		return "A bot"
	}

	// Fallback 2: display name from Bitbucket API.
	apiUser, err := bitbucket.UsersGet(ctx, accountID, "")
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Bitbucket user info",
			slog.Any("error", err), slog.String("account_id", accountID))
		return accountID // Last resort: return the original Bitbucket account ID.
	}

	if apiUser.DisplayName != "" {
		// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
		_ = data.UpsertUser(ctx, "", apiUser.DisplayName, accountID, "", "", "")
		return apiUser.DisplayName
	}

	return accountID // Last resort: return the original Bitbucket account ID (ugly but unavoidable).
}
