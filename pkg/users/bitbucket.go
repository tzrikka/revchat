package users

import (
	"fmt"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
)

// BitbucketToSlackRef converts Bitbucket account ID into a Slack user reference.
// This depends on the user's email address being the same in both systems.
// This function returns the display name if the user is not found in Slack.
func BitbucketToSlackRef(ctx workflow.Context, cmd *cli.Command, accountID, displayName string) string {
	id := bitbucketToSlackID(ctx, cmd, accountID)
	if id != "" {
		return fmt.Sprintf("<@%s>", id)
	}

	if displayName == "" {
		displayName = "Display Name"
	}

	return displayName
}

// bitbucketToSlackID converts a Bitbucket account ID into a Slack user ID.
// This depends on the user's email address being the same in both systems.
// This function returns an empty string if the account ID is not found.
func bitbucketToSlackID(ctx workflow.Context, cmd *cli.Command, accountID string) string {
	l := workflow.GetLogger(ctx)

	email, err := data.BitbucketUserEmailByID(accountID)
	if err != nil {
		l.Error("failed to load Bitbucket user email", "error", err, "account_id", accountID)
		return ""
	}

	// Note: unlike GitHub, we can't see user emails unless we know them in advance.

	if email == "" || email == "bot" {
		return ""
	}

	id, err := data.SlackUserIDByEmail(email)
	if err != nil {
		l.Error("failed to load Slack user ID", "error", err, "email", email)
	}

	if id == "" {
		id = EmailToSlackID(ctx, cmd, email)
	}

	return id
}
