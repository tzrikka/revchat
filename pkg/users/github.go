package users

import (
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/github"
)

// GitHubIDToEmail converts a GitHub username into an email address. This function returns an empty
// string if the username is not found. It uses persistent data storage, or API calls as a fallback.
func GitHubIDToEmail(ctx workflow.Context, username string) string {
	if username == "" {
		return ""
	}

	if user := data.SelectUserByGitHubID(ctx, username); user.Email != "" {
		return user.Email
	}

	// Use GitHub API as a fallback to get the user's details.
	ghUser, err := github.UsersGetByUsername(ctx, username)
	if err != nil {
		logger.From(ctx).Warn("failed to retrieve GitHub user info",
			slog.Any("error", err), slog.String("username", username))
		return ""
	}

	if ghUser.Email == "" {
		logger.From(ctx).Warn("found GitHub user but without an email address", slog.String("username", username))
		return ""
	}

	// Don't return an error here (i.e. abort the calling workflow) - we have a result, even if we failed to save it.
	email := strings.ToLower(ghUser.Email)
	_ = data.UpsertUser(ctx, email, "", username, "", "", "")

	return email
}

// GitHubIDToSlackID converts a GitHub username into a Slack user ID. This
// function returns an empty string if the username is not found, or if it belongs
// to a GitHub team. It uses persistent data storage, or API calls as a fallback.
func GitHubIDToSlackID(ctx workflow.Context, username string, checkOptIn bool) string {
	// Don't even check GitHub teams, only individual users.
	if strings.Contains(username, "/") {
		return ""
	}

	user := data.SelectUserByGitHubID(ctx, username)
	if user.SlackID == "" {
		// Workaround in case only the user's GitHub account ID isn't stored yet, but the rest is.
		user = data.SelectUserByEmail(ctx, GitHubIDToEmail(ctx, username))
	}

	if user.SlackID == "" {
		return ""
	}

	if checkOptIn && !user.IsOptedIn() {
		return ""
	}

	return user.SlackID
}

// GitHubIDToSlackRef converts a GitHub user into a Slack user reference. This function returns
// a GitHub profile link (in Slack markdown format) if the user is not found in Slack, or if
// it's a GitHub bot/team. It uses persistent data storage, or API calls as a fallback.
func GitHubIDToSlackRef(ctx workflow.Context, username, url, userType string) string {
	if !strings.Contains(username, "/") && userType != "Bot" {
		if id := GitHubIDToSlackID(ctx, username, false); id != "" {
			return fmt.Sprintf("<@%s>", id)
		}
	}

	// Fallback: if there's no Slack profile, linkify the GitHub user profile.
	return fmt.Sprintf("<%s?preview=no|@%s>", url, username)
}
