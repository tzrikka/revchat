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
func EmailToBitbucketID(ctx workflow.Context, email string) string {
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

// BitbucketActorToEmail is a trivial wrapper around [BitbucketIDToEmail]. Merely syntactic sugar.
func BitbucketActorToEmail(ctx workflow.Context, actor bitbucket.User) string {
	return BitbucketIDToEmail(ctx, actor.AccountID, actor.Type)
}

// BitbucketIDToEmail converts a Bitbucket account ID into an email address. This function returns an empty string if the
// account ID is not found, and "bot" for non-user app accounts. It uses persistent data storage, or API calls as a fallback.
func BitbucketIDToEmail(ctx workflow.Context, accountID, accountType string) string {
	if user := data.SelectUserByBitbucketID(ctx, accountID); user.Email != "" {
		return user.Email
	}

	if accountType == "app_user" {
		return "bot" // Not a real user, so no point to look-up in Jira.
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
	_ = data.UpsertUser(ctx, email, "", accountID, "", "", "")

	return email
}

// BitbucketActorToSlackID is a trivial wrapper around [BitbucketIDToSlackID].
// It avoids unnecessary API calls for non-user accounts by checking the account type first.
func BitbucketActorToSlackID(ctx workflow.Context, actor bitbucket.User, checkOptIn bool) string {
	if actor.Type == "app_user" {
		return ""
	}
	return BitbucketIDToSlackID(ctx, actor.AccountID, checkOptIn)
}

// BitbucketIDToSlackID converts a Bitbucket account ID into a Slack user ID. This function returns an empty
// string if the account ID is not found. It uses persistent data storage, or API calls as a fallback.
func BitbucketIDToSlackID(ctx workflow.Context, accountID string, checkOptIn bool) string {
	user := data.SelectUserByBitbucketID(ctx, accountID)
	if user.SlackID == "" {
		// Workaround in case only the user's Bitbucket account ID isn't stored yet, but the rest is.
		user = data.SelectUserByEmail(ctx, BitbucketIDToEmail(ctx, accountID, "user"))
	}

	if checkOptIn && !user.IsOptedIn() {
		return ""
	}

	return user.SlackID
}

// BitbucketIDToSlackRef converts a Bitbucket account ID into a Slack user mention. This function returns a
// display name if the account ID is not found. It uses persistent data storage, or API calls as a fallback.
func BitbucketIDToSlackRef(ctx workflow.Context, accountID, displayName string) string {
	user := data.SelectUserByBitbucketID(ctx, accountID)
	if user.SlackID == "" {
		// Workaround in case only the user's Bitbucket account ID isn't stored yet, but the rest is.
		user = data.SelectUserByEmail(ctx, BitbucketIDToEmail(ctx, accountID, "user"))
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
	apiUser, err := bitbucket.UsersGetByAccountID(ctx, accountID)
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
