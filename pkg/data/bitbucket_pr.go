package data

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

// StoreBitbucketPR writes a snapshot of a Bitbucket PR, which is used to detect metadata changes.
func StoreBitbucketPR(ctx workflow.Context, url string, pr any) {
	f, err := os.OpenFile(prPath(url), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 // Verified URL.
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
	f, err := os.Open(prPath(url)) //gosec:disable G304 // URL received from verified 3rd-party.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		logger.From(ctx).Error("failed to open Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
		return nil, err
	}
	defer f.Close()

	m := map[string]any{}
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		logger.From(ctx).Error("failed to read Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
		return nil, err
	}

	return m, nil
}

func DeleteBitbucketPR(ctx workflow.Context, url string) {
	if ctx == nil { // For unit testing.
		_ = deletePRFileActivity(context.Background(), prPath(url), "")
		return
	}

	if err := executeLocalActivity(ctx, deletePRFileActivity, nil, prPath(url), ""); err != nil {
		logger.From(ctx).Warn("failed to delete Bitbucket PR snapshot", slog.Any("error", err), slog.String("pr_url", url))
	}
}

// prPath returns the absolute path to the JSON snapshot file of a Bitbucket PR.
// This function is different from [dataPath] because it supports subdirectories.
// It creates any necessary parent directories, but not the file itself.
func prPath(url string) string {
	prefix, _ := xdg.CreateDir(xdg.DataHome, config.DirName)
	suffix := strings.TrimPrefix(url, "https://")
	filePath := filepath.Clean(filepath.Join(prefix, suffix))

	_ = os.MkdirAll(filepath.Dir(filePath), xdg.NewDirectoryPermissions)

	return filePath + "_snapshot.json"
}
