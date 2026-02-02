package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

const (
	BitbucketPRSnapshotFileSuffix = "_snapshot.json"
)

// StoreBitbucketPR writes a snapshot of a Bitbucket PR, which is used to detect metadata changes.
func StoreBitbucketPR(ctx workflow.Context, url string, pr any) {
	path, err := cachedDataPath(url + BitbucketPRSnapshotFileSuffix)
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR snapshot path", slog.Any("error", err), slog.String("pr_url", url))
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 // Verified URL, suffix is hardcoded.
	if err != nil {
		logger.From(ctx).Error("failed to open Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
		return
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	if err := e.Encode(pr); err != nil {
		logger.From(ctx).Error("failed to write Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
	}
}

// LoadBitbucketPR reads a snapshot of a Bitbucket PR, which is used to detect metadata
// changes. If a snapshot doesn't exist, this function returns a nil map and no error.
func LoadBitbucketPR(ctx workflow.Context, url string) (map[string]any, error) {
	path, err := cachedDataPath(url + BitbucketPRSnapshotFileSuffix)
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR snapshot path", slog.Any("error", err), slog.String("pr_url", url))
		return nil, err
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		logger.From(ctx).Error("failed to open Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
		return nil, err
	}
	defer f.Close()

	m := map[string]any{}
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		logger.From(ctx).Error("failed to read Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
		return nil, fmt.Errorf("failed to read Bitbucket PR snapshot: %w", err)
	}

	if len(m) == 0 {
		return nil, nil
	}
	return m, nil
}

func DeleteBitbucketPR(ctx workflow.Context, url string) {
	if ctx == nil { // For unit testing.
		_ = deletePRFileActivity(context.Background(), url+BitbucketPRSnapshotFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, deletePRFileActivity, nil, url+BitbucketPRSnapshotFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
	}
}
