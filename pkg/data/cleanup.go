package data

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

// FullPRCleanup deletes all the data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall need to clean up.
func FullPRCleanup(ctx workflow.Context, channelID, prURL string) {
	if err := LogSlackChannelArchived(channelID, prURL); err != nil {
		logger.From(ctx).Error("failed to log Slack channel archived", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("pr_url", prURL))
	}
	if prURL == "" {
		return
	}

	if err := DeleteBitbucketDiffstat(prURL); err != nil {
		logger.From(ctx).Error("failed to delete Bitbucket PR diffstat",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
	if err := DeleteBitbucketPR(prURL); err != nil {
		logger.From(ctx).Error("failed to delete Bitbucket PR snapshot",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
	if err := DeleteBitbucketBuilds(prURL); err != nil {
		logger.From(ctx).Error("failed to delete Bitbucket PR build status",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
	if err := DeleteTurns(prURL); err != nil {
		logger.From(ctx).Error("failed to delete Bitbucket PR turn-taking state",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
	if err := DeleteURLAndIDMapping(prURL); err != nil {
		logger.From(ctx).Error("failed to delete PR URL / Slack channel mappings",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
}
