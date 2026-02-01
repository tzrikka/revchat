package github

import (
	"log/slog"
	"slices"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

// InitPRData saves the initial state of a new PR: snapshots of PR metadata,
// and a 2-way ID mapping for syncs between GitHub and Slack. If there are
// errors, they are logged but ignored, as we can try to recreate the data later.
func InitPRData(ctx workflow.Context, event PullRequestEvent, prChannelID, slackAlertsChannel string) {
	if err := data.MapURLAndID(ctx, event.PullRequest.HTMLURL, prChannelID); err != nil {
		_ = activities.AlertError(ctx, slackAlertsChannel, "failed to set mapping between a PR and its Slack channel",
			err, "PR URL", event.PullRequest.HTMLURL, "Slack channel ID", prChannelID)
	}

	email := users.GitHubIDToEmail(ctx, event.Sender.Login)
	if email == "" {
		logger.From(ctx).Error("initializing GitHub PR data without author's email",
			slog.String("pr_url", event.PullRequest.HTMLURL), slog.String("login", event.Sender.Login))
		activities.AlertWarn(ctx, slackAlertsChannel, "Failed to determine a PR author's email address!",
			"GitHub login", event.Sender.Login)
		return
	}

	data.InitTurns(ctx, event.PullRequest.HTMLURL, email)
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
