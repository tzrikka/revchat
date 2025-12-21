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
		logger.Error(ctx, "failed to log Slack channel archived", err,
			slog.String("channel_id", channelID), slog.String("pr_url", prURL))
	}
	if prURL == "" {
		return
	}

	if err := DeleteBitbucketDiffstat(prURL); err != nil {
		logger.Error(ctx, "failed to delete Bitbucket PR diffstat", err, slog.String("pr_url", prURL))
	}
	if err := DeleteBitbucketPR(prURL); err != nil {
		logger.Error(ctx, "failed to delete Bitbucket PR snapshot", err, slog.String("pr_url", prURL))
	}
	if err := DeleteBitbucketBuilds(prURL); err != nil {
		logger.Error(ctx, "failed to delete Bitbucket PR build status", err, slog.String("pr_url", prURL))
	}
	if err := DeleteTurns(prURL); err != nil {
		logger.Error(ctx, "failed to delete Bitbucket PR turn-taking state", err, slog.String("pr_url", prURL))
	}
	if err := DeleteURLAndIDMapping(prURL); err != nil {
		logger.Error(ctx, "failed to delete PR URL / Slack channel mappings", err, slog.String("pr_url", prURL))
	}
}
