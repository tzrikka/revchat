package data

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data/internal"
)

type (
	CommitStatus = internal.CommitStatus
	PRStatus     = internal.PRStatus
)

func ReadBitbucketBuilds(ctx workflow.Context, prURL string) PRStatus {
	if ctx == nil { // For unit testing.
		status, err := internal.ReadBitbucketBuilds(context.Background(), prURL) //workflowcheck:ignore
		if err != nil {
			return PRStatus{}
		}
		return *status
	}

	status := PRStatus{}
	err := executeLocalActivity(ctx, internal.ReadBitbucketBuilds, &status, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read Bitbucket PR's build states",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return PRStatus{}
	}

	return status
}

// UpdateBitbucketBuilds appends the given build status to the given PR,
// unless the new status is based on a different (i.e. newer) commit,
// in which case this function discards all previous build statuses.
func UpdateBitbucketBuilds(ctx workflow.Context, prURL, commitHash, key string, cs CommitStatus) {
	if ctx == nil { // For unit testing.
		_ = internal.UpdateBitbucketBuilds(context.Background(), prURL, commitHash, key, cs) //workflowcheck:ignore
		return
	}

	if err := executeLocalActivity(ctx, internal.UpdateBitbucketBuilds, nil, prURL, commitHash, key, cs); err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR's build states", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("commit_hash", commitHash))
	}
}

func DeleteBitbucketBuilds(ctx workflow.Context, prURL string) {
	if ctx == nil { // For unit testing.
		_ = internal.DeleteGenericPRFile(context.Background(), prURL+internal.BuildsFileSuffix) //workflowcheck:ignore
		return
	}

	if err := executeLocalActivity(ctx, internal.DeleteGenericPRFile, nil, prURL+internal.BuildsFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete Bitbucket PR's build states",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
}
