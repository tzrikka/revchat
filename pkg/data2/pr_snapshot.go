package data2

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data2/internal"
)

const (
	PRSnapshotFileSuffix = "_snapshot.json"
)

// StorePRSnapshot writes a snapshot of a PR, which is used to detect and analyze metadata changes.
func StorePRSnapshot(ctx workflow.Context, url string, pr any) {
	if ctx == nil { // For unit testing.
		_ = internal.StorePRSnapshot(context.Background(), url+PRSnapshotFileSuffix, pr)
		return
	}

	if err := executeLocalActivity(ctx, internal.StorePRSnapshot, nil, url+PRSnapshotFileSuffix, pr); err != nil {
		logger.From(ctx).Error("failed to store PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
	}
}

// LoadPRSnapshot reads a snapshot of a PR, which is used to detect and analyze metadata
// changes. If a snapshot doesn't exist, this function returns a nil map and no error.
func LoadPRSnapshot(ctx workflow.Context, url string) (map[string]any, error) {
	if ctx == nil { // For unit testing.
		return internal.LoadPRSnapshot(context.Background(), url+PRSnapshotFileSuffix)
	}

	pr := map[string]any{}
	if err := executeLocalActivity(ctx, internal.LoadPRSnapshot, &pr, url+PRSnapshotFileSuffix); err != nil {
		logger.From(ctx).Error("failed to load PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
		return nil, err
	}

	return pr, nil
}

func DeletePRSnapshot(ctx workflow.Context, url string) {
	if ctx == nil { // For unit testing.
		_ = internal.DeletePRFile(context.Background(), url+PRSnapshotFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, internal.DeletePRFile, nil, url+PRSnapshotFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
	}
}
