package users

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

// EmailToSlackID looks up a Slack user based on the given email address.
// This function assumes a mapping does not exist, so it adds it if found.
func EmailToSlackID(ctx workflow.Context, cmd *cli.Command, email string) string {
	user := slackLookupUserByEmailActivity(ctx, cmd, email)
	if user == nil {
		return ""
	}

	if err := data.AddSlackUser(user.ID, email); err != nil {
		log.Error(ctx, "failed to save Slack user ID/email mapping", "error", err, "user_id", user.ID, "email", email)
	}

	return user.ID
}

// https://docs.slack.dev/reference/methods/users.lookupByEmail
type slackUsersLookupByEmailRequest struct {
	Email string `json:"email"`
}

// https://docs.slack.dev/reference/methods/users.lookupByEmail
type slackUsersLookupByEmailResponse struct {
	slackResponse

	User *slackUser `json:"user,omitempty"`
}

// https://docs.slack.dev/reference/objects/user-object/.
// A more complete version can be found in "pkg/slack/users.go".
type slackUser struct {
	ID       string `json:"id"`
	TeamID   string `json:"team_id"`
	RealName string `json:"real_name"`
	IsBot    bool   `json:"is_bot"`
}

type slackResponse struct {
	OK               bool              `json:"ok"`
	Error            string            `json:"error,omitempty"`
	Needed           string            `json:"needed,omitempty"`   // Scope errors (undocumented).
	Provided         string            `json:"provided,omitempty"` // Scope errors (undocumented).
	Warning          string            `json:"warning,omitempty"`
	ResponseMetadata *responseMetadata `json:"response_metadata,omitempty"`
}

type responseMetadata struct {
	Messages   []string `json:"messages,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

// https://docs.slack.dev/reference/methods/users.lookupByEmail
func slackLookupUserByEmailActivity(ctx workflow.Context, cmd *cli.Command, email string) *slackUser {
	req := slackUsersLookupByEmailRequest{Email: email}
	a := executeTimpaniActivity(ctx, cmd, "slack.users.lookupByEmail", req)

	resp := slackUsersLookupByEmailResponse{}
	if err := a.Get(ctx, &resp); err != nil {
		log.Error(ctx, "failed to lookup Slack user by email", "error", err, "email", email)
		return nil
	}

	return resp.User
}
