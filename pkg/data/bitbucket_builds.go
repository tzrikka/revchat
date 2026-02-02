package data

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

// PRStatus represents the current status of all reported
// builds for a specific Bitbucket PR at a specific commit.
type PRStatus struct {
	CommitHash string                  `json:"commit_hash"`
	Builds     map[string]CommitStatus `json:"builds"`
}

type CommitStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Desc  string `json:"desc"`
	URL   string `json:"url"`
}

const (
	buildsFileSuffix = "_builds.json"
)

var prBuildsMutexes RWMutexMap

func ReadBitbucketBuilds(ctx workflow.Context, prURL string) PRStatus {
	mu := prBuildsMutexes.Get(prURL)
	mu.RLock()
	defer mu.RUnlock()

	if ctx == nil { // For unit testing.
		pr, err := readBitbucketBuildsActivity(context.Background(), prURL)
		if err != nil {
			return PRStatus{}
		}
		return *pr
	}

	pr := new(PRStatus)
	err := executeLocalActivity(ctx, readBitbucketBuildsActivity, pr, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read Bitbucket PR's build states",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return PRStatus{}
	}
	return *pr
}

// UpdateBitbucketBuilds appends the given build status to the given PR,
// unless the new status is based on a different (i.e. newer) commit,
// in which case this function discards all previous build statuses.
func UpdateBitbucketBuilds(ctx workflow.Context, prURL, commitHash, key string, cs CommitStatus) {
	mu := prBuildsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	if ctx == nil { // For unit testing.
		_ = updateBitbucketBuildsActivity(context.Background(), prURL, commitHash, key, cs)
		return
	}

	if err := executeLocalActivity(ctx, updateBitbucketBuildsActivity, nil, prURL, commitHash, key, cs); err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR's build states", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("commit_hash", commitHash))
	}
}

func DeleteBitbucketBuilds(ctx workflow.Context, prURL string) {
	mu := prBuildsMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	if ctx == nil { // For unit testing.
		_ = deletePRFileActivity(context.Background(), prURL+buildsFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, deletePRFileActivity, nil, prURL+buildsFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete Bitbucket PR's build states",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

// readBitbucketBuildsActivity runs as a Temporal local activity (using
// [executeLocalActivity]), and expects the caller to hold the appropriate mutex.
func readBitbucketBuildsActivity(_ context.Context, url string) (*PRStatus, error) {
	return readBitbucketBuildsFile(url)
}

// updateBitbucketBuildsActivity runs as a Temporal local activity (using
// [executeLocalActivity]) and expects the caller to hold the appropriate mutex.
func updateBitbucketBuildsActivity(_ context.Context, url, commitHash, key string, cs CommitStatus) error {
	pr, err := readBitbucketBuildsFile(url)
	if err != nil {
		return err
	}

	if pr.CommitHash != commitHash {
		pr.CommitHash = commitHash
		pr.Builds = map[string]CommitStatus{}
	}

	pr.Builds[key] = cs
	path, err := cachedDataPath(url + buildsFileSuffix)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(pr)
}

func readBitbucketBuildsFile(url string) (*PRStatus, error) {
	path, err := cachedDataPath(url + buildsFileSuffix)
	if err != nil {
		return nil, err
	}

	pr := new(PRStatus)
	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return pr, nil
		}
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&pr); err != nil {
		return nil, err
	}

	return pr, nil
}
