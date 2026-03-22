package data

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data2"
)

// CleanupPRData deletes all the data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall need to clean up.
func CleanupPRData(ctx workflow.Context, channelID, prURL string) {
	LogSlackChannelArchived(ctx, channelID, prURL)
	if prURL == "" {
		return
	}

	data2.DeleteBitbucketBuilds(ctx, prURL)
	DeleteDiffstat(ctx, prURL)
	data2.DeletePRSnapshot(ctx, prURL)
	DeleteTurns(ctx, prURL)

	DeleteURLAndIDMapping(ctx, prURL)
}
