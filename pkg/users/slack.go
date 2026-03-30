package users

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

var (
	displayNameCache = cache.New[string](4*time.Hour, cache.DefaultCleanupInterval)
	iconCache        = cache.New[string](4*time.Hour, cache.DefaultCleanupInterval)

	// No need for thread safety here: this is set only once per process, and even
	// if multiple workflows set it concurrently, the value will be the same anyway.
	workspaceURL = ""
)

// EmailToSlackID retrieves a Slack user's ID based on their email address. This function returns an
// empty string if the user ID is not found. It uses persistent data storage, or API calls as a fallback.
func EmailToSlackID(ctx workflow.Context, email string) string {
	if email == "" || email == "bot" {
		return ""
	}

	if user := data.SelectUserByEmail(ctx, email); user.SlackID != "" {
		return user.SlackID
	}

	slackUser, err := slack.UsersLookupByEmail(ctx, email)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info", slog.Any("error", err), slog.String("email", email))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	_ = data.UpsertUser(ctx, email, slackUser.RealName, "", "", slackUser.ID, "")
	displayNameCache.Set(slackUser.ID, slackUser.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(slackUser.ID, slackUser.Profile.Image48, cache.DefaultExpiration)

	return slackUser.ID
}

// SlackIDToEmail converts a Slack user's ID into their email address. This function returns an empty
// string if the user ID is not found. It uses persistent data storage, or API calls as a fallback.
func SlackIDToEmail(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, _, _ := data.SelectUserBySlackID(ctx, userID)
	if user.Email != "" {
		return user.Email
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info", slog.Any("error", err), slog.String("user_id", userID))
		return ""
	}

	if info.IsBot {
		// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
		_ = data.UpsertUser(ctx, "bot", "", "", "", userID, "")
		return "bot"
	}

	email := strings.ToLower(info.Profile.Email)
	if email == "" {
		logger.From(ctx).Error("Slack user has no email address",
			slog.String("user_id", userID), slog.String("real_name", info.RealName))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	_ = data.UpsertUser(ctx, email, info.RealName, "", "", userID, "")
	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return email
}

// SlackIDToIcon retrieves a Slack user's icon path based on their ID.
// This function uses ephemeral data caching, or API calls as a fallback.
func SlackIDToIcon(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	if icon, found := iconCache.Get(userID); found {
		return icon
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info", slog.Any("error", err), slog.String("user_id", userID))
		return ""
	}

	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return info.Profile.Image48
}

// SlackIDToDisplayName retrieves a Slack user's display name based on their ID.
// This function uses ephemeral data caching, or API calls as a fallback.
// Note that display names are not permanent like [SlackIDToRealName].
func SlackIDToDisplayName(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	if displayName, found := displayNameCache.Get(userID); found {
		return "@" + displayName
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info", slog.Any("error", err), slog.String("user_id", userID))
		return ""
	}

	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return "@" + info.Profile.DisplayName
}

// SlackIDToRealName retrieves a Slack user's full name based on their ID.
// This function uses persistent data storage, or API calls as a fallback.
// Note that real names are permanent unlike [SlackIDToDisplayName].
func SlackIDToRealName(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, _, _ := data.SelectUserBySlackID(ctx, userID)
	if user.RealName != "" {
		return user.RealName
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", userID))
		return ""
	}

	email := strings.ToLower(info.Profile.Email)
	if info.IsBot {
		email = "bot"
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	_ = data.UpsertUser(ctx, email, info.RealName, "", "", userID, "")
	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return info.RealName
}

// SlackMentionToBitbucketRef converts a Slack user mention ("<@U123>") into a Bitbucket
// account ID ("@{account:uuid}"). This function returns the user's full/display name if
// the user is not found. It uses persistent data storage, or API calls as a fallback.
func SlackMentionToBitbucketRef(ctx workflow.Context, slackUserRef string) string {
	userID := slackUserRef[2 : len(slackUserRef)-1]
	user, _, _ := data.SelectUserBySlackID(ctx, userID)
	if user.BitbucketID == "" {
		// Workaround in case only the user's Slack ID isn't stored yet, but the rest is.
		user = data.SelectUserByEmail(ctx, SlackIDToEmail(ctx, userID))
	}

	if user.BitbucketID != "" {
		return fmt.Sprintf("@{%s}", user.BitbucketID)
	}

	// Fallback 1: slower but better, based on the user's email address.
	if accountID := EmailToBitbucketID(ctx, SlackIDToEmail(ctx, userID)); accountID != "" {
		return fmt.Sprintf("@{%s}", accountID)
	}

	if workspaceURL == "" {
		if resp, err := slack.AuthTest(ctx); err == nil {
			workspaceURL = resp.URL
		}
	}

	// Fallback 2: return the Slack user's full name, as a link to their profile.
	if realName := SlackIDToRealName(ctx, userID); realName != "" {
		return fmt.Sprintf("[%s](%steam/%s)", realName, workspaceURL, userID)
	}

	// Fallback 3: return the Slack user's display name, as a link to their profile.
	if displayName := SlackIDToDisplayName(ctx, userID); displayName != "" {
		return fmt.Sprintf("[%s](%steam/%s)", displayName, workspaceURL, userID)
	}

	return slackUserRef // Last resort: return the original Slack user mention (ugly but unavoidable).
}
