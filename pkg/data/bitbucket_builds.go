package data

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
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
	statusFileSuffix = "_status"
)

var prStatusMutexes RWMutexMap

func ReadBitbucketBuilds(ctx workflow.Context, prURL string) *PRStatus {
	mu := prStatusMutexes.Get(prURL)
	mu.RLock()
	defer mu.RUnlock()

	pr := new(PRStatus)
	err := executeLocalActivity(ctx, readBitbucketBuildsActivity, pr, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to read Bitbucket PR's build states",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return nil
	}

	return pr
}

// UpdateBitbucketBuilds appends the given build status to the given PR,
// unless the new status is based on a different (i.e. newer) commit,
// in which case this function discards all previous build statuses.
func UpdateBitbucketBuilds(ctx workflow.Context, prURL, commitHash, key string, cs CommitStatus) {
	mu := prStatusMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	if err := executeLocalActivity(ctx, updateBitbucketBuildsActivity, nil, prURL, commitHash, key, cs); err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR's build states", slog.Any("error", err),
			slog.String("pr_url", prURL), slog.String("commit_hash", commitHash))
	}
}

func DeleteBitbucketBuilds(ctx workflow.Context, prURL string) {
	mu := prStatusMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	if ctx == nil { // For unit testing.
		_ = deletePRFileActivity(context.Background(), prURL, statusFileSuffix)
		return
	}

	if err := executeLocalActivity(ctx, deletePRFileActivity, nil, prURL, statusFileSuffix); err != nil {
		logger.From(ctx).Warn("failed to delete Bitbucket PR's build states",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}
}

// readBitbucketBuildsActivity runs as a Temporal local activity (using
// [executeLocalActivity]), and expects the caller to hold the appropriate mutex.
func readBitbucketBuildsActivity(_ context.Context, url string) (*PRStatus, error) {
	return readStatusFile(url)
}

// updateBitbucketBuildsActivity runs as a Temporal local activity (using
// [executeLocalActivity]) and expects the caller to hold the appropriate mutex.
func updateBitbucketBuildsActivity(ctx context.Context, url, commitHash, key string, cs CommitStatus) error {
	pr, err := readStatusFile(url)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		pr = &PRStatus{CommitHash: "not found"}
	}

	if pr.CommitHash != commitHash {
		pr.CommitHash = commitHash
		pr.Builds = map[string]CommitStatus{}
	}

	pr.Builds[key] = cs
	path, err := cachedDataPath(url, statusFileSuffix)
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
	return e.Encode(pr)
}

func readStatusFile(url string) (*PRStatus, error) {
	path, err := cachedDataPath(url, statusFileSuffix)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party.
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pr := new(PRStatus)
	if err := json.NewDecoder(f).Decode(&pr); err != nil {
		return nil, err
	}

	return pr, nil
}
