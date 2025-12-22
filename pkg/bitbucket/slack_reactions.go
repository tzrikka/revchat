package bitbucket

import (
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// addOKReaction adds the ":ok:" emoji as a reaction to the
// Slack message identified by the given Bitbucket comment URL.
func addOKReaction(ctx workflow.Context, url string) {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve PR comment's Slack IDs",
			slog.Any("error", err), slog.String("bitbucket_url", url))
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		logger.From(ctx).Warn("can't add reaction to Slack message - missing/bad IDs",
			slog.String("bitbucket_url", url), slog.String("slack_ids", ids))
		return
	}

	_ = slack.ReactionsAdd(ctx, id[0], id[len(id)-1], "ok")
}

// removeOKReaction removes the ":ok:" emoji from the Slack bot's reactions
// in the Slack message identified by the given Bitbucket comment URL.
func removeOKReaction(ctx workflow.Context, url string) {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve PR comment's Slack IDs",
			slog.Any("error", err), slog.String("bitbucket_url", url))
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		logger.From(ctx).Error("can't remove reaction from Slack message - missing/bad IDs",
			slog.String("bitbucket_url", url), slog.String("slack_ids", ids))
		return
	}

	_ = slack.ReactionsRemove(ctx, id[0], id[len(id)-1], "ok")
}
