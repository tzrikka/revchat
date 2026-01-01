package data

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"

	"go.temporal.io/sdk/workflow"
)

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

var prStatusMutexes RWMutexMap

func ReadBitbucketBuilds(ctx workflow.Context, url string) *PRStatus {
	mu := prStatusMutexes.Get(url)
	mu.RLock()
	defer mu.RUnlock()

	pr := new(PRStatus)
	err := executeLocalActivity(ctx, readBitbucketBuildsActivity, pr, url)
	if err != nil {
		return nil
	}

	return pr
}

func UpdateBitbucketBuilds(ctx workflow.Context, url, commitHash, key string, cs CommitStatus) error {
	mu := prStatusMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	return executeLocalActivity(ctx, updateBitbucketBuildsActivity, nil, url, commitHash, key, cs)
}

func DeleteBitbucketBuilds(url string) error {
	mu := prStatusMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	path, err := cachedDataPath(url, "_status")
	if err != nil {
		return err
	}

	return os.Remove(path) //gosec:disable G304 // URL received from signature-verified 3rd-party.
}

// readBitbucketBuildsActivity runs as a local activity and expects the caller to hold the appropriate mutex.
func readBitbucketBuildsActivity(_ context.Context, url string) (*PRStatus, error) {
	return readStatusFile(url)
}

// updateBitbucketBuildsActivity runs as a local activity and expects the caller to hold the appropriate mutex.
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
	path, err := cachedDataPath(url, "_status")
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
	path, err := cachedDataPath(url, "_status")
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
