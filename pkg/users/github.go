package users

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
)

// GitHubToSlackID converts a GitHub username into a Slack user ID. This depends on
// the user's email address being the same in both systems. This function returns an
// empty string if the username is not found, or if it belongs to a GitHub team or app.
func GitHubToSlackID(ctx workflow.Context, cmd *cli.Command, username string, checkOptIn bool) string {
	// Don't even check GitHub teams, only individual users.
	if strings.Contains(username, "/") {
		return ""
	}

	user, err := data.SelectUserByGitHubID(username)
	if err != nil {
		logger.Error(ctx, "failed to load user by GitHub ID", err, slog.String("username", username))
		return ""
	}

	if checkOptIn && user.ThrippyLink == "" {
		return ""
	}

	return user.SlackID
}

// GitHubToSlackRef converts GitHub user details into a Slack user reference.
// This depends on the user's email address being the same in both systems.
// This function returns a GitHub profile link (in Slack markdown format)
// if the user is not found in Slack, or if it belongs to a GitHub team or app.
func GitHubToSlackRef(ctx workflow.Context, cmd *cli.Command, username, url string) string {
	// Don't even check GitHub teams, only individual users.
	if strings.Contains(username, "/") {
		return fmt.Sprintf("<%s|@%s>", url, username)
	}

	if id := GitHubToSlackID(ctx, cmd, username, false); id != "" {
		return fmt.Sprintf("<@%s>", id)
	}

	// Fallback: if there's no Slack user ID, linkify the GitHub user profile.
	return fmt.Sprintf("<%s|@%s>", url, username)
}
