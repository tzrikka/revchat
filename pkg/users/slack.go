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
	email, _ := SlackIDToEmail(ctx, userID)
	accountID, _ := EmailToBitbucketID(ctx, bitbucketWorkspace, email)
	if accountID != "" {
		return fmt.Sprintf("@{%s}", accountID)
	}

	user, err := slack.UsersInfoActivity(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return slackUserRef // Fallback: return the original Slack user reference.
	}

	return user.RealName
}

// SlackIDToEmail retrieves a Slack user's email address based
// on their ID. This function uses data caching, and API calls as
// a fallback. Not finding an email address is considered an error.
func SlackIDToEmail(ctx workflow.Context, userID string) (string, error) {
	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", userID)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if user.Email != "" {
		return user.Email, nil
	}

	slackUser, err := slack.UsersInfoActivity(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return "", err
	}

	if slackUser.IsBot {
		if err := data.UpsertUser("bot", "", "", userID, ""); err != nil {
			log.Error(ctx, "failed to save Slack bot ID mapping", "error", err, "user_id", userID, "email", "bot")
		}
		return "bot", nil
	}

	if slackUser.Profile.Email == "" {
		log.Error(ctx, "Slack user has no email address", "user_id", userID, "real_name", slackUser.RealName)
		return "", fmt.Errorf("slack user has no email address in their profile: %s", userID)
	}

	email := slackUser.Profile.Email
	if err := data.UpsertUser(email, "", "", userID, ""); err != nil {
		log.Error(ctx, "failed to save Slack user ID/email", "error", err, "user_id", userID, "email", email)
	}

	return email, nil
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
	}

	slackUser, err := slack.UsersLookupByEmailActivity(ctx, email)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "email", email)
		return ""
	}

	if err := data.UpsertUser(email, "", "", slackUser.ID, ""); err != nil {
		log.Error(ctx, "failed to save Slack user ID/email", "error", err, "user_id", slackUser.ID, "email", email)
	}

	return slackUser.ID
}
