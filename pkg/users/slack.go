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
	email, err := data.SlackUserEmailByID(userID)
	if err != nil {
		log.Error(ctx, "failed to load Slack user email", "error", err, "user_id", userID)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if email != "" {
		return email, nil
	}

	user, err := slack.UsersInfoActivity(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return "", err
	}

	if user.IsBot {
		if err := data.AddSlackUser(userID, "bot"); err != nil {
			log.Error(ctx, "failed to save Slack bot ID mapping", "error", err, "user_id", userID, "email", "bot")
		}
		return "bot", nil
	}

	if user.Profile.Email == "" {
		log.Error(ctx, "Slack user has no email address", "user_id", userID, "real_name", user.RealName)
		return "", fmt.Errorf("slack user has no email address in their profile: %s", userID)
	}

	email = user.Profile.Email
	if err := data.AddSlackUser(userID, email); err != nil {
		log.Error(ctx, "failed to save Slack user ID/email", "error", err, "user_id", userID, "email", email)
	}

	return email, nil
}

// EmailToSlackID retrieves a Slack user's ID based on their email address.
// This function uses data caching, and API calls as a fallback.
func EmailToSlackID(ctx workflow.Context, email string) string {
	id, err := data.SlackUserIDByEmail(email)
	if err != nil {
		log.Error(ctx, "failed to load Slack user ID", "error", err, "email", email)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if id != "" {
		return id
	}

	user, err := slack.UsersLookupByEmailActivity(ctx, email)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "email", email)
		return ""
	}

	if err := data.AddSlackUser(user.ID, email); err != nil {
		log.Error(ctx, "failed to save Slack user ID/email", "error", err, "user_id", user.ID, "email", email)
	}

	return user.ID
}
