package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	fileFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	filePerms = xdg.NewFilePermissions
)

// MutexMap is a concurrency-safe map of string keys to *sync.Mutex values. The zero value is ready to use.
// This is useful for managing concurrent access to multiple files, where each file is identified by a string key.
// We don't use [sync.RWMutex] because even "read" operations may call [fixEmptyJSONFile], which modifies the file.
type MutexMap struct {
	sm sync.Map
}

func (m *MutexMap) Get(key string) *sync.Mutex {
	actual, _ := m.sm.LoadOrStore(key, &sync.Mutex{})
	return actual.(*sync.Mutex) //nolint:errcheck // Safe type assertion, always succeeds by definition.
}

var dataFileMutexes MutexMap

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
	case strings.HasSuffix(path, DiffstatFileSuffix):
		fallthrough
	case strings.HasSuffix(path, "/users.json"):
		_ = os.WriteFile(path, []byte("[]\n"), xdg.NewFilePermissions)
	case strings.HasSuffix(path, ".json"):
		_ = os.WriteFile(path, []byte("{}\n"), xdg.NewFilePermissions)
	}
}

// writeGenericJSONFile writes the given map as JSON to the specified file. It expects the caller to hold the appropriate mutex.
func writeGenericJSONFile(_ context.Context, filename string, data any) error {
	path, err := dataPath(filename)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // Path specified by admin, or from signature-verified event.
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(data)
}

// DeleteGenericPRFile deletes a file related to a specific PR. Unlike [os.Remove],
// this is idempotent: it does not return an error if the file does not exist.
func DeleteGenericPRFile(_ context.Context, prURLWithSuffix string) error {
	mu := dataFileMutexes.Get(prURLWithSuffix)
	mu.Lock()
	defer mu.Unlock()

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
