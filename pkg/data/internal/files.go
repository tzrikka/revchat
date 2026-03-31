package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	fileFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	filePerms = xdg.NewFilePermissions
)

// dataFileMutexMap is a package-level, concurrency-safe cache that maps string keys to [sync.Mutex] pointers,
// but with time-based expiration and garbage collection, unlike [sync.Map]. This is useful for managing
// concurrent access to multiple files, where each file is identified by a string key. We don't use
// [sync.RWMutex] because even "read" operations may call [fixEmptyJSONFile], which modifies the file.
var dataFileMutexMap = cache.New[*sync.Mutex](24*time.Hour, cache.DefaultCleanupInterval)

// getDataFileMutex returns a mutex for the given key, creating it if it doesn't exist. The mutex is cached with an expiration time
// to prevent unbounded growth of the map, which is refreshed on each call to ensure that active keys remain in the map.
// The caller should hold the returned mutex while accessing the file associated with the key.
func getDataFileMutex(key string) *sync.Mutex {
	mu := &sync.Mutex{}
	if !dataFileMutexMap.Add(key, mu, cache.DefaultExpiration) {
		mu, _ = dataFileMutexMap.Get(key)
		dataFileMutexMap.Set(key, mu, cache.DefaultExpiration) // Refresh expiration time on access.
	}
	return mu
}

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

// readGenericJSONFile expects the caller to hold the appropriate mutex.
func readGenericJSONFile(filename string) (map[string]string, error) {
	path, err := dataPath(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get data file path: %w", err)
	}

	f, err := os.Open(path) //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var m map[string]string
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, fmt.Errorf("failed to read/decode JSON: %w", err)
	}

	return m, nil
}

// writeGenericJSONFile expects the caller to hold the appropriate mutex.
func writeGenericJSONFile(filename string, data any) error {
	path, err := dataPath(filename)
	if err != nil {
		return fmt.Errorf("failed to get data file path: %w", err)
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // Path specified by admin, or from signature-verified event.
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(data)
}

// DeleteGenericPRFile deletes a file related to a specific PR. Unlike [os.Remove],
// this is idempotent: it does not return an error if the file does not exist.
func DeleteGenericPRFile(_ context.Context, prURLWithSuffix string) error {
	mu := getDataFileMutex(prURLWithSuffix)
	mu.Lock()
	defer mu.Unlock()

	path, err := dataPath(prURLWithSuffix)
	if err != nil {
		return fmt.Errorf("failed to get data file path: %w", err)
	}

	if err := os.Remove(path); err != nil { //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
