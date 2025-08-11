package users

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
)

// GitHubToSlackRef converts GitHub user details into a Slack user reference.
// This depends on the user's email address being the same in both systems.
// This function returns a GitHub profile link (in Slack markdown format)
// if the user is not found in Slack, or if it belongs to a GitHub team or app.
func GitHubToSlackRef(ctx workflow.Context, cmd *cli.Command, username, url string) string {
	id := GitHubToSlackID(ctx, cmd, username, false)
	if id != "" {
		return fmt.Sprintf("<@%s>", id)
	}

	// Fallback: if there's no Slack user ID, linkify the GitHub user profile.
	return fmt.Sprintf("<%s|@%s>", url, username)
}

// GitHubToSlackID converts a GitHub username into a Slack user ID. This depends on
// the user's email address being the same in both systems. This function returns an
// empty string if the username is not found, or if it belongs to a GitHub team or app.
func GitHubToSlackID(ctx workflow.Context, cmd *cli.Command, username string, checkOptIn bool) string {
	l := workflow.GetLogger(ctx)

	// Don't even check GitHub teams, only individual users.
	if strings.Contains(username, "/") {
		return ""
	}

	email, err := data.GitHubUserEmailByID(username)
	if err != nil {
		l.Error("failed to load GitHub user email", "error", err, "username", username)
		return ""
	}

	if email == "" || email == "bot" {
		return ""
	}

	if checkOptIn {
		optedIn, err := data.IsOptedIn(email)
		if err != nil {
			l.Error("failed to load user opt-in status", "error", err, "email", email)
			return ""
		}
		if !optedIn {
			return ""
		}
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
