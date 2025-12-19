package data

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// FullPRCleanup deletes all the data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall need to clean up.
func FullPRCleanup(ctx workflow.Context, channelID, prURL string) {
	if err := LogSlackChannelArchived(channelID, prURL); err != nil {
		log.Error(ctx, "failed to log Slack channel archived", "error", err, "channel_id", channelID, "pr_url", prURL)
	}
	if prURL == "" {
		return
	}

	if err := DeleteBitbucketDiffstat(prURL); err != nil {
		log.Error(ctx, "failed to delete Bitbucket PR diffstat", "error", err, "pr_url", prURL)
	}
	if err := DeleteBitbucketPR(prURL); err != nil {
		log.Error(ctx, "failed to delete Bitbucket PR snapshot", "error", err, "pr_url", prURL)
	}
	if err := DeleteBitbucketBuilds(prURL); err != nil {
		log.Error(ctx, "failed to delete Bitbucket PR build status", "error", err, "pr_url", prURL)
	}
	if err := DeleteTurns(prURL); err != nil {
		log.Error(ctx, "failed to delete Bitbucket PR turn-taking state", "error", err, "pr_url", prURL)
	}
	if err := DeleteURLAndIDMapping(prURL); err != nil {
		log.Error(ctx, "failed to delete PR URL / Slack channel mappings", "error", err, "pr_url", prURL)
	}
}
