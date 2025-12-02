package users

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
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
// This function uses data caching, and API calls as a fallback.
func SlackIDToEmail(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", userID)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if user.Email != "" {
		return user.Email
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return ""
	}

	if info.IsBot {
		if err := data.UpsertUser("bot", "", "", userID, "", "", ""); err != nil {
			log.Error(ctx, "failed to save Slack bot ID mapping", "error", err, "user_id", userID, "email", "bot")
		}
		return "bot"
	}

	email := info.Profile.Email
	if email == "" {
		log.Error(ctx, "Slack user has no email address", "user_id", userID, "real_name", info.RealName)
		return ""
	}

	if err := data.UpsertUser(email, "", "", userID, info.RealName, info.Profile.DisplayName, ""); err != nil {
		log.Error(ctx, "failed to save Slack user details", "error", err, "user_id", userID, "email", email)
	}

	return email
}

// SlackIDToRealName retrieves a Slack user's full name based on their ID.
// This function uses data caching, and API calls as a fallback.
func SlackIDToRealName(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", userID)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if user.RealName != "" {
		return user.RealName
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return ""
	}

	email := info.Profile.Email
	if info.IsBot {
		email = "bot"
	}

	realName := info.RealName
	if err := data.UpsertUser(email, "", "", userID, realName, info.Profile.DisplayName, ""); err != nil {
		log.Error(ctx, "failed to save Slack user details", "error", err, "user_id", userID, "real_name", realName)
	}

	return realName
}

// SlackIDToDisplayName retrieves a Slack user's display name based on their ID.
// This function uses data caching, and API calls as a fallback.
func SlackIDToDisplayName(ctx workflow.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", userID)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if user.SlackName != "" {
		return "@" + user.SlackName
	}

	info, err := slack.UsersInfo(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return ""
	}

	email := info.Profile.Email
	if info.IsBot {
		email = "bot"
	}

	displayName := info.Profile.DisplayName
	if err := data.UpsertUser(email, "", "", userID, info.RealName, displayName, ""); err != nil {
		log.Error(ctx, "failed to save Slack user details", "error", err, "user_id", userID, "disp_name", displayName)
	}

	if displayName != "" {
		displayName = "@" + displayName
	}
	return displayName
}

// EmailToSlackID retrieves a Slack user's ID based on their email address.
// This function uses data caching, and API calls as a fallback.
func EmailToSlackID(ctx workflow.Context, email string) string {
	if email == "" || email == "bot" {
		return ""
	}

	user, err := data.SelectUserByEmail(email)
	if err != nil {
		log.Error(ctx, "failed to load user by email", "error", err, "email", email)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if user.SlackID != "" {
		return user.SlackID
	} else if user.Email == email {
		log.Debug(ctx, "user in DB without Slack ID", "email", email)
	}

	slackUser, err := slack.UsersLookupByEmail(ctx, email)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "email", email)
		return ""
	}

	profile := slackUser.Profile
	if err := data.UpsertUser(email, "", "", slackUser.ID, slackUser.RealName, profile.DisplayName, ""); err != nil {
		log.Error(ctx, "failed to save Slack user details", "error", err, "user_id", slackUser.ID, "email", email)
	}

	return slackUser.ID
}
