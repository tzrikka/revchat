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
	displayNameCache = cache.New(24*time.Hour, cache.DefaultCleanupInterval)
	iconCache        = cache.New(24*time.Hour, cache.DefaultCleanupInterval)
)

// SlackToBitbucketRef converts a Slack user reference ("<@U123>") into a Bitbucket account
// ID ("@{account:uuid}"). This depends on the user's email address being the same in both
// systems. This function returns the Slack display name if the user is not found.
func SlackToBitbucketRef(ctx workflow.Context, bitbucketWorkspace, slackUserRef string) string {
	userID := slackUserRef[2 : len(slackUserRef)-1]
	accountID, _ := EmailToBitbucketID(ctx, bitbucketWorkspace, SlackIDToEmail(ctx, userID))
	if accountID != "" {
		return fmt.Sprintf("@{%s}", accountID)
	}

	if realName := SlackIDToRealName(ctx, userID); realName != "" {
		return realName
	}

	return slackUserRef // Fallback: return the original Slack user reference.
}

// SlackIDToEmail retrieves a Slack user's email address based on their ID.
// This function uses persistent data storage, and API calls as a fallback.
func SlackIDToEmail(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID",
			slog.Any("error", err), slog.String("user_id", userID))
		// Don't return the error (i.e. abort the calling workflow) - use the Slack API as a fallback.
	}
	if user.Email != "" {
		return user.Email
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", userID))
		return ""
	}

	if info.IsBot {
		if err := data.UpsertUser("bot", "", "", userID, "", ""); err != nil {
			logger.From(ctx).Error("failed to save Slack bot ID mapping", slog.Any("error", err),
				slog.String("user_id", userID), slog.String("email", "bot"))
		}
		return "bot"
	}

	email := strings.ToLower(info.Profile.Email)
	if email == "" {
		logger.From(ctx).Error("Slack user has no email address",
			slog.String("user_id", userID), slog.String("real_name", info.RealName))
		return ""
	}

	if err := data.UpsertUser(email, "", "", userID, info.RealName, ""); err != nil {
		logger.From(ctx).Error("failed to save Slack user details", slog.Any("error", err),
			slog.String("user_id", userID), slog.String("email", email))
		// Don't return the error (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	}
	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return email
}

// SlackIDToRealName retrieves a Slack user's full name based on their ID.
// This function uses persistent data storage, and API calls as a fallback.
func SlackIDToRealName(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID",
			slog.Any("error", err), slog.String("user_id", userID))
		// Don't abort - try to use the Slack API as a fallback.
	}
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

	realName := info.RealName
	if err := data.UpsertUser(email, "", "", userID, realName, ""); err != nil {
		logger.From(ctx).Error("failed to save Slack user details", slog.Any("error", err),
			slog.String("user_id", userID), slog.String("real_name", realName))
		// Don't return the error (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	}
	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return realName
}

// SlackIDToDisplayName retrieves a Slack user's display name based on their ID.
// This function uses ephemeral data caching, and API calls as a fallback.
func SlackIDToDisplayName(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	if displayName, found := displayNameCache.Get(userID); found {
		return "@" + displayName
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", userID))
		return ""
	}

	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return "@" + info.Profile.DisplayName
}

// SlackIDToIcon retrieves a Slack user's icon path based on their ID.
// This function uses ephemeral data caching, and API calls as a fallback.
func SlackIDToIcon(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	if icon, found := iconCache.Get(userID); found {
		return icon
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", userID))
		return ""
	}

	displayNameCache.Set(userID, info.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(userID, info.Profile.Image48, cache.DefaultExpiration)

	return info.Profile.Image48
}

// EmailToSlackID retrieves a Slack user's ID based on their email address.
// This function uses data caching, and API calls as a fallback.
func EmailToSlackID(ctx workflow.Context, email string) string {
	if email == "" || email == "bot" {
		return ""
	}

	user, err := data.SelectUserByEmail(email)
	if err != nil {
		logger.From(ctx).Error("failed to load user by email",
			slog.Any("error", err), slog.String("email", email))
		// Don't abort - try to use the Slack API as a fallback.
	}
	if user.SlackID != "" {
		return user.SlackID
	} else if user.Email == email {
		logger.From(ctx).Debug("user in DB without Slack ID", slog.String("email", email))
	}

	slackUser, err := slack.UsersLookupByEmail(ctx, email)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("email", email))
		return ""
	}

	if err := data.UpsertUser(email, "", "", slackUser.ID, slackUser.RealName, ""); err != nil {
		logger.From(ctx).Error("failed to save Slack user details", slog.Any("error", err),
			slog.String("user_id", slackUser.ID), slog.String("email", email))
		// Don't return the error (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	}
	displayNameCache.Set(slackUser.ID, slackUser.Profile.DisplayName, cache.DefaultExpiration)
	iconCache.Set(slackUser.ID, slackUser.Profile.Image48, cache.DefaultExpiration)

	return slackUser.ID
}
