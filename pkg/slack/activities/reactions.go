package activities

import (
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// AddOKReaction adds the ":ok:" emoji as a reaction to the
// Slack message identified by the given PR comment URL.
func AddOKReaction(ctx workflow.Context, url string) {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve PR comment's Slack IDs",
			slog.Any("error", err), slog.String("comment_url", url))
		return
	}

	parts := strings.Split(ids, "/")
	if len(parts) < 2 {
		logger.From(ctx).Warn("can't add reaction to Slack message - missing/bad IDs",
			slog.String("comment_url", url), slog.String("slack_ids", ids))
		return
	}

	_ = slack.ReactionsAdd(ctx, parts[0], parts[len(parts)-1], "ok")
}

// RemoveOKReaction removes the ":ok:" emoji from the Slack bot's reactions
// in the Slack message identified by the given PR comment URL.
func RemoveOKReaction(ctx workflow.Context, url string) {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve PR comment's Slack IDs",
			slog.Any("error", err), slog.String("comment_url", url))
		return
	}

	parts := strings.Split(ids, "/")
	if len(parts) < 2 {
		logger.From(ctx).Error("can't remove reaction from Slack message - missing/bad IDs",
			slog.String("comment_url", url), slog.String("slack_ids", ids))
		return
	}

	_ = slack.ReactionsRemove(ctx, parts[0], parts[len(parts)-1], "ok")
}
