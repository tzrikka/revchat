package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	activityTimeout  = 3 * time.Second
	activityAttempts = 3

	fileFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	filePerms = xdg.NewFilePermissions
)

// pathCache caches the absolute paths to data files to avoid repeated filesystem operations.
// Entries expire after 7 days, and the cache is cleaned every about once a day (1,399
// is a prime number of minutes close to 23.5 hours to avoid a repeated spike pattern).
var pathCache = cache.New[string](7*24*time.Hour, 1433*time.Minute)

func executeLocalActivity(ctx workflow.Context, activity, result any, args ...any) error {
	f := runtime.FuncForPC(reflect.ValueOf(activity).Pointer())
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		StartToCloseTimeout: activityTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 1.0,
			MaximumAttempts:    activityAttempts,
		},
	})

	start := time.Now()
	err := workflow.ExecuteLocalActivity(ctx, activity, args...).Get(ctx, result)
	logger.From(ctx).Debug("executed local Temporal activity for data access", slog.String("activity", f.Name()),
		slog.Duration("duration", time.Since(start)), slog.Any("error", err))

	return err
}

// readJSONActivity runs as a Temporal local activity (using
// [executeLocalActivity]), and expects the caller to hold the appropriate mutex.
func readJSONActivity(_ context.Context, filename string) (map[string]string, error) {
	path, err := cachedDataPath(filename)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path) //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var m map[string]string
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}

	return m, nil
}

// writeJSONActivity runs as a Temporal local activity (using
// [executeLocalActivity]), and expects the caller to hold the appropriate mutex.
func writeJSONActivity(_ context.Context, filename string, m map[string]string) error {
	path, err := cachedDataPath(filename)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(m)
}

// deletePRFileActivity deletes a file related to a specific PR. Unlike [os.Remove],
// this function is idempotent: it does not return an error if the file does not exist.
func deletePRFileActivity(_ context.Context, prURLWithSuffix string) error {
	path, err := cachedDataPath(prURLWithSuffix)
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

// cachedDataPath returns the absolute path to a data file for the given relative path.
// The relative path can be a filename, or a PR's URL with a file-content-type suffix.
// This function creates the file and any parent directories if they don't exist yet.
// The resulting absolute path is cached to minimize filesystem I/O operations.
func cachedDataPath(relativePath string) (string, error) {
	if path, found := pathCache.Get(relativePath); found {
		return path, nil
	}

	path, err := xdg.CreateFilePath(xdg.DataHome, config.DirName, strings.TrimPrefix(relativePath, "https://"))
	if err != nil {
		return "", fmt.Errorf("failed to create data file path: %w", err)
	}
	fixEmptyJSONFile(path)

	pathCache.Set(relativePath, path, cache.DefaultExpiration)
	return path, nil
}

// fixEmptyJSONFile checks if the given path points to an empty JSON file,
// and if so, writes an appropriate empty JSON structure to it (either "{}"
// or "[]"). This is useful because empty files can't be decoded as JSON,
// but are a possible initial state after calling [cachedDataPath].
// This function ignores any errors, as it's just a best-effort fix.
func fixEmptyJSONFile(path string) {
	file, err := os.Stat(path)
	if err != nil {
		return
	}
	if file.Size() > 0 {
		return
	}

	switch {
	case strings.HasSuffix(path, diffstatFileSuffix):
		_ = os.WriteFile(path, []byte("[]\n"), xdg.NewFilePermissions)
	case strings.HasSuffix(path, ".json"):
		_ = os.WriteFile(path, []byte("{}\n"), xdg.NewFilePermissions)
	}
}

type RWMutexMap struct {
	sm sync.Map
}

func (mm *RWMutexMap) Get(key string) *sync.RWMutex {
	actual, _ := mm.sm.LoadOrStore(key, &sync.RWMutex{})
	return actual.(*sync.RWMutex) //nolint:errcheck // Type conversion always succeeds.
}
