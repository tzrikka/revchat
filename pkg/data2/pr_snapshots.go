package data2

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data2/internal"
)

// StorePRSnapshot writes a snapshot of a PR, which is used to detect and analyze metadata changes.
func StorePRSnapshot(ctx workflow.Context, prURL string, pr any) {
	if ctx == nil { // For unit testing.
		_ = internal.StorePRSnapshot(context.Background(), prURL, pr)
		return
	}

	if err := executeLocalActivity(ctx, internal.StorePRSnapshot, nil, prURL, pr); err != nil {
		logger.From(ctx).Error("failed to store PR snapshot", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

// LoadPRSnapshot reads a snapshot of a PR, which is used to detect and analyze metadata
// changes. If a snapshot doesn't exist, this function returns a nil map and no error.
func LoadPRSnapshot(ctx workflow.Context, prURL string) (map[string]any, error) {
	if ctx == nil { // For unit testing.
		return internal.LoadPRSnapshot(context.Background(), prURL)
	}

	var pr map[string]any
	if err := executeLocalActivity(ctx, internal.LoadPRSnapshot, &pr, prURL); err != nil {
		logger.From(ctx).Error("failed to load PR snapshot", slog.Any("error", err), slog.String("pr_url", prURL))
		return nil, err
	}

	return pr, nil
}

// FindPRsByCommit returns all (0 or more) the PR snapshots that are currently associated with the given commit hash.
func FindPRsByCommit(ctx workflow.Context, hash string) ([]map[string]any, error) {
	if ctx == nil { // For unit testing.
		return internal.FindPRsByCommit(context.Background(), hash)
	}

	var prs []map[string]any
	if err := executeLocalActivity(ctx, internal.FindPRsByCommit, &prs, hash); err != nil {
		logger.From(ctx).Error("failed to find PR snapshots by commit hash", slog.Any("error", err), slog.String("hash", hash))
		return nil, err
	}

	return prs, nil
}

func DeletePRSnapshot(ctx workflow.Context, prURL string) {
	if ctx == nil { // For unit testing.
		_ = internal.DeleteGenericPRFile(context.Background(), prURL+internal.PRSnapshotFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, internal.DeleteGenericPRFile, nil, prURL+internal.PRSnapshotFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete PR snapshot", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}
