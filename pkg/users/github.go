package users

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
)

// GitHubToSlackID converts a GitHub username into a Slack user ID. This depends on
// the user's email address being the same in both systems. This function returns an
// empty string if the username is not found, or if it belongs to a GitHub team or app.
func GitHubToSlackID(ctx workflow.Context, username string, checkOptIn bool) string {
	// Don't even check GitHub teams, only individual users.
	if strings.Contains(username, "/") {
		return ""
	}

	user := data.SelectUserByGitHubID(ctx, username)
	if user.SlackID == "" {
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
func GitHubToSlackRef(ctx workflow.Context, username, url string) string {
	// Don't even check GitHub teams, only individual users.
	if strings.Contains(username, "/") {
		return fmt.Sprintf("<%s|@%s>", url, username)
	}

	if id := GitHubToSlackID(ctx, username, false); id != "" {
		return fmt.Sprintf("<@%s>", id)
	}

	// Fallback: if there's no Slack user ID, linkify the GitHub user profile.
	return fmt.Sprintf("<%s|@%s>", url, username)
}
