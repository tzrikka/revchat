package data

import (
	"go.temporal.io/sdk/workflow"
)

// CleanupPRData deletes all the data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall need to clean up.
func CleanupPRData(ctx workflow.Context, channelID, prURL string) {
	if channelID != "" {
		LogSlackChannelArchived(ctx, channelID, prURL)
		if prURL == "" {
			DeleteURLAndIDMapping(ctx, channelID)
		}
	}
	if prURL == "" {
		return
	}

	DeleteBitbucketBuilds(ctx, prURL)
	DeleteDiffstat(ctx, prURL)
	DeletePRSnapshot(ctx, prURL)
	DeleteTurns(ctx, prURL)

	DeleteURLAndIDMapping(ctx, prURL)
}
