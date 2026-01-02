package data

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"slices"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

const (
	diffstatFileSuffix = "_diffstat"
)

var prDiffstatMutexes RWMutexMap

func ReadBitbucketDiffstatLength(url string) int {
	return len(diffstatPaths(readBitbucketDiffstat(url)))
}

func ReadBitbucketDiffstatPaths(url string) []string {
	return diffstatPaths(readBitbucketDiffstat(url))
}

func UpdateBitbucketDiffstat(url string, ds []bitbucket.Diffstat) error {
	mu := prDiffstatMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	path, err := cachedDataPath(url, diffstatFileSuffix)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // URL received from signature-verified 3rd-party.
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(ds)
}

func DeleteDiffstat(ctx workflow.Context, url string) {
	mu := prDiffstatMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	if ctx == nil { // For unit testing.
		_ = deletePRFileActivity(context.Background(), url, diffstatFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, deletePRFileActivity, nil, url, diffstatFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete PR diffstat", slog.Any("error", err), slog.String("pr_url", url))
	}
}

func readBitbucketDiffstat(url string) []bitbucket.Diffstat {
	mu := prDiffstatMutexes.Get(url)
	mu.RLock()
	defer mu.RUnlock()

	path, err := cachedDataPath(url, diffstatFileSuffix)
	if err != nil {
		return nil
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party.
	if err != nil {
		return nil
	}
	defer f.Close()

	ds := []bitbucket.Diffstat{}
	if err := json.NewDecoder(f).Decode(&ds); err != nil {
		return nil
	}

	return ds
}

func diffstatPaths(ds []bitbucket.Diffstat) []string {
	var paths []string
	for _, d := range ds {
		if d.New != nil {
			paths = append(paths, d.New.Path)
		}
		if d.Old != nil {
			paths = append(paths, d.Old.Path)
		}
	}

	slices.Sort(paths)
	return slices.Compact(paths)
}
