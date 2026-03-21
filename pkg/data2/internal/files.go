package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

// dataPath returns the absolute path to a data file with the given relative path.
// The relative path can be a filename, or a PR's URL with a file-content-type suffix.
// This function creates the file and any parent directories if they don't exist yet.
func dataPath(urlWithSuffix string) (string, error) {
	path, err := xdg.CreateFilePath(xdg.DataHome, config.DirName, strings.TrimPrefix(urlWithSuffix, "https://"))
	if err != nil {
		return "", fmt.Errorf("failed to create data file path: %w", err)
	}

	fixEmptyJSONFile(path)
	return path, nil
}

// fixEmptyJSONFile checks if the given path points to an empty JSON file.
// If so, it writes an appropriate empty JSON structure to it (either "{}"
// or "[]"). This is useful because empty files can't be decoded as JSON,
// but are a possible initial state when calling [dataPath]. This
// function ignores any errors, as it's just a best-effort fix.
func fixEmptyJSONFile(path string) {
	file, err := os.Stat(path)
	if err != nil {
		return
	}
	if file.Size() > 0 {
		return
	}

	switch {
	case strings.HasSuffix(path, bitbucketDiffstatFileSuffix):
		fallthrough
	case strings.HasSuffix(path, "/users.json"):
		_ = os.WriteFile(path, []byte("[]\n"), xdg.NewFilePermissions)
	case strings.HasSuffix(path, ".json"):
		_ = os.WriteFile(path, []byte("{}\n"), xdg.NewFilePermissions)
	}
}

// DeletePRFile deletes a file related to a specific PR. Unlike [os.Remove], this
// function is idempotent: it does not return an error if the file does not exist.
func DeletePRFile(_ context.Context, prURLWithSuffix string) error {
	path, err := dataPath(prURLWithSuffix)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil { //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	return nil
}
