package github

import (
	"log/slog"
	"slices"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
)

// InitPRData saves the initial state of a new PR: mainly a 2-way ID mapping for syncs between GitHub
// and Slack. If there are errors, they are logged but ignored, as we can try to recreate the data later.
func InitPRData(ctx workflow.Context, event PullRequestEvent, channelID string) {
	_ = data.MapURLAndID(ctx, event.PullRequest.HTMLURL, channelID)

	email := users.GitHubIDToEmail(ctx, event.Sender.Login)
	if email == "" {
		logger.From(ctx).Error("initializing GitHub PR data without author's email",
			slog.String("pr_url", event.PullRequest.HTMLURL), slog.String("login", event.Sender.Login))
	} else {
		data.InitTurns(ctx, event.PullRequest.HTMLURL, email)
	}
}

func userLogins(us []User) []string {
	if len(us) == 0 {
		return nil
	}

	logins := make([]string, 0, len(us))
	for _, u := range us {
		if u.Type == "User" {
			logins = append(logins, u.Login)
		}
	}

	slices.Sort(logins)
	return slices.Compact(logins)
}
