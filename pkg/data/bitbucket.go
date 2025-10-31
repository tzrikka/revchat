package data

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

// StoreBitbucketPR saves a snapshot of a Bitbucket pull request.
// This is used to detect changes in the pull request over time.
func StoreBitbucketPR(url string, pr any) error {
	f, err := os.OpenFile(prPath(url), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- verified URL
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(pr)
}

// LoadBitbucketPR loads a snapshot of a Bitbucket pull request.
// This is used to detect changes in the pull request over time.
// This function returns nil if no snapshot is found.
func LoadBitbucketPR(url string) (map[string]any, error) {
	f, err := os.Open(prPath(url)) //gosec:disable G304 -- URL received from verified 3rd-party
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	m := map[string]any{}
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}

	return m, nil
}

// DeleteBitbucketPR deletes the snapshot of a Bitbucket pull request when it
// becomes obsolete (i.e. when the PR is merged, closed, or marked as a draft).
// This function is idempotent: it does not return an error if the snapshot does not exist.
func DeleteBitbucketPR(url string) error {
	if err := os.Remove(prPath(url)); err != nil { //gosec:disable G304 -- URL received from verified 3rd-party
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

// dataPath returns the absolute path to the given data file.
// It also creates an empty file if it doesn't already exist.
func dataPath(filename string) string {
	path, _ := xdg.CreateFile(xdg.DataHome, config.DirName, filename)
	return path
}

func prPath(url string) string {
	prefix, _ := xdg.CreateDir(xdg.DataHome, config.DirName)
	suffix, _ := strings.CutPrefix(url, "https://")
	filePath := filepath.Clean(filepath.Join(prefix, suffix))

	_ = os.MkdirAll(filepath.Dir(filePath), xdg.NewDirectoryPermissions)

	return filePath
}
