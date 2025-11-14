package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
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

func ReadBitbucketBuilds(url string) *PRStatus {
	mu := prStatusMutexes.Get(url)
	mu.RLock()
	defer mu.RUnlock()

	pr, err := readStatusFile(url)
	if err != nil {
		return nil
	}

	return pr
}

func SummarizeBitbucketBuilds(url string) string {
	mu := prStatusMutexes.Get(url)
	mu.RLock()
	defer mu.RUnlock()

	pr, err := readStatusFile(url)
	if err != nil {
		return ""
	}

	keys := slices.Sorted(maps.Keys(pr.Builds))
	var summary []string
	for _, k := range keys {
		switch s := pr.Builds[k].State; s {
		case "INPROGRESS":
			// Don't show in-progress builds in summary.
		case "SUCCESSFUL":
			summary = append(summary, "large_green_circle")
		default: // "FAILED", "STOPPED".
			summary = append(summary, "red_circle")
		}
	}

	// Returns a sequence of space-separated emoji.
	if len(summary) > 0 {
		return fmt.Sprintf(":%s:", strings.Join(summary, ": :"))
	}
	return ""
}

func UpdateBitbucketBuilds(prURL, commitHash, key string, cs CommitStatus) error {
	mu := prStatusMutexes.Get(prURL)
	mu.Lock()
	defer mu.Unlock()

	pr, err := readStatusFile(prURL)
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
	return writeStatusFile(prURL, pr)
}

// readStatusFile expects the caller to hold the appropriate mutex.
func readStatusFile(url string) (*PRStatus, error) {
	path, err := cachedDataPath(url, "_status")
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path) //gosec:disable G304 -- URL received from signature-verified 3rd-party
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

// writeStatusFile expects the caller to hold the appropriate mutex.
func writeStatusFile(url string, pr *PRStatus) error {
	path, err := cachedDataPath(url, "_status")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 -- URL received from signature-verified 3rd-party
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(pr)
}
