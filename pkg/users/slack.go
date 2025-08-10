package users

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
)

// EmailToSlackID looks up a Slack user based on the given email address.
// This function assumes a mapping does not exist, so it adds it if found.
func EmailToSlackID(ctx workflow.Context, cmd *cli.Command, email string) string {
	user := slackUsersLookupByEmailActivity(ctx, cmd, email)
	if user == nil {
		return ""
	}

	id := user["id"].(string)
	if err := data.AddSlackUser(id, email); err != nil {
		msg := "failed to save Slack user ID/email mapping"
		workflow.GetLogger(ctx).Error(msg, "error", err, "user_id", id, "email", email)
	}

	return id
}
