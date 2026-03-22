package data2

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data2/internal"
)

const (
	DiffstatFileSuffix = internal.DiffstatFileSuffix
)

func ReadDiffstatPaths(ctx workflow.Context, prURL string) []string {
	if ctx == nil { // For unit testing.
		paths, err := internal.ReadDiffstatPaths(context.Background(), prURL)
		if err != nil {
			return nil
		}
		return paths
	}

	paths := []string{}
	err := executeLocalActivity(ctx, internal.ReadDiffstatPaths, &paths, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read PR diffstat", slog.Any("error", err), slog.String("pr_url", prURL))
		return nil
	}

	return paths
}

func UpdateDiffstat(ctx workflow.Context, prURL string, files any) {
	if ctx == nil { // For unit testing.
		_ = internal.UpdateDiffstat(context.Background(), prURL, files)
		return
	}

	if err := executeLocalActivity(ctx, internal.UpdateDiffstat, nil, prURL, files); err != nil {
		logger.From(ctx).Error("failed to update PR diffstat", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

func DeleteDiffstat(ctx workflow.Context, prURL string) {
	if ctx == nil { // For unit testing.
		_ = internal.DeleteGenericPRFile(context.Background(), prURL+internal.DiffstatFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, internal.DeleteGenericPRFile, nil, prURL+internal.DiffstatFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete PR diffstat", slog.Any("error", err), slog.String("pr_url", prURL))
	}
}
