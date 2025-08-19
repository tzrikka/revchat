package data

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	bitbucketFile = "bitbucket_prs.json"
)

// StoreBitbucketPR saves a snapshot of a Bitbucket pull request.
// This is used to detect changes in the pull request over time.
func StoreBitbucketPR(url string, pr any) error {
	path := dataPath(bitbucketFile)

	m, err := readBitbucketPRs(path)
	if err != nil {
		return err
	}

	m[url] = pr
	return writeBitbucketPRs(path, m)
}

// LoadBitbucketPR loads a snapshot of a Bitbucket pull request.
// This is used to detect changes in the pull request over time.
func LoadBitbucketPR(url string) (any, error) {
	m, err := readBitbucketPRs(dataPath(bitbucketFile))
	if err != nil {
		return nil, err
	}

	return m[url], nil
}

// DeleteBitbucketPR deletes the snapshot of a Bitbucket pull request when it
// becomes obsolete (i.e. when the PR is merged, closed, or marked as a draft).
func DeleteBitbucketPR(url string) error {
	path := dataPath(bitbucketFile)

	m, err := readBitbucketPRs(path)
	if err != nil {
		return err
	}

	delete(m, url)
	return writeBitbucketPRs(path, m)
}

// dataPath returns the absolute path to the given data file.
// It also creates an empty file if it doesn't already exist.
func dataPath(filename string) string {
	path, _ := xdg.CreateFile(xdg.DataHome, config.DirName, filename)
	return path
}

func readBitbucketPRs(path string) (map[string]any, error) {
	f, err := os.ReadFile(path) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return nil, err
	}

	// Special case: empty files can't be parsed as JSON,
	// but this initial state is valid.
	m := map[string]any{}
	if len(f) == 0 {
		return m, nil
	}

	if err := json.NewDecoder(bytes.NewReader(f)).Decode(&m); err != nil {
		return nil, err
	}

	return m, nil
}

func writeBitbucketPRs(path string, m map[string]any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(m)
}
