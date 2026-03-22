package internal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
)

const (
	BuildsFileSuffix = "_builds.json"
)

type CommitStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Desc  string `json:"desc"`
	URL   string `json:"url"`
}

// PRStatus represents the current status of all reported
// builds for a specific Bitbucket PR at a specific commit.
type PRStatus struct {
	CommitHash string                  `json:"commit_hash"`
	Builds     map[string]CommitStatus `json:"builds"`
}

func ReadBitbucketBuilds(_ context.Context, prURL string) (*PRStatus, error) {
	mu := dataFileMutexes.Get(prURL + BuildsFileSuffix)
	mu.Lock() // Not RLock() because [readBitbucketBuildsFile] may call [fixEmptyJSONFile].
	defer mu.Unlock()

	return readBitbucketBuildsFile(prURL)
}

func UpdateBitbucketBuilds(ctx context.Context, prURL, commitHash, key string, cs CommitStatus) error {
	mu := dataFileMutexes.Get(prURL + BuildsFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	status, err := readBitbucketBuildsFile(prURL)
	if err != nil {
		return err
	}

	if status.CommitHash != commitHash {
		status.CommitHash = commitHash
		status.Builds = map[string]CommitStatus{}
	}
	status.Builds[key] = cs

	return writeGenericJSONFile(ctx, prURL+BuildsFileSuffix, status)
}

// readBitbucketBuildsFile expects the calling function to hold the appropriate mutex for the given PR URL.
func readBitbucketBuildsFile(prURL string) (*PRStatus, error) {
	path, err := dataPath(prURL + BuildsFileSuffix)
	if err != nil {
		return nil, err
	}

	status := new(PRStatus)
	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status, nil
		}
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&status); err != nil {
		return nil, err
	}
	return status, nil
}
