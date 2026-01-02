package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
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
	path, err := cachedDataPath(filename, "")
	if err != nil {
		return nil, err
	}

	// Special case: empty files can't be parsed as JSON, but this initial state is valid.
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() == 0 {
		return map[string]string{}, nil
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
	path, err := cachedDataPath(filename, "")
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
func deletePRFileActivity(_ context.Context, prURL, suffix string) error {
	path := prURL
	if suffix != "" {
		var err error
		path, err = cachedDataPath(prURL, suffix)
		if err != nil {
			return err
		}
	}

	if err := os.Remove(path); err != nil { //gosec:disable G304 // URL received from verified 3rd-party, suffix is hardcoded.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	return nil
}

func cachedDataPath(filename, suffix string) (string, error) {
	path, found := pathCache.Get(filename + suffix)
	if found {
		return path, nil
	}

	// Special handling for per-PR files.
	if strings.HasPrefix(filename, "https://") {
		path := urlBasedPath(filename, suffix)
		pathCache.Set(filename+suffix, path, cache.DefaultExpiration)
		return path, nil
	}

	// Sanitize the filename, create the directory and the file if needed.
	path, err := xdg.CreateFile(xdg.DataHome, config.DirName, filename)
	if err != nil {
		return "", fmt.Errorf("failed to create data file: %w", err)
	}

	pathCache.Set(filename+suffix, path, cache.DefaultExpiration)
	return path, nil
}

// urlBasedPath returns the absolute path to a JSON file related to a specific PR.
// This function is different from [xdg.CreateFile] because it supports subdirectories.
// It creates any necessary parent directories, but not the file itself.
func urlBasedPath(url, suffix string) string {
	prefix, _ := xdg.CreateDir(xdg.DataHome, config.DirName)
	subdirs := strings.TrimPrefix(url, "https://")
	filePath := filepath.Clean(filepath.Join(prefix, subdirs))

	_ = os.MkdirAll(filepath.Dir(filePath), xdg.NewDirectoryPermissions)

	return fmt.Sprintf("%s%s.json", filePath, suffix)
}

type RWMutexMap struct {
	sm sync.Map
}

func (mm *RWMutexMap) Get(key string) *sync.RWMutex {
	actual, _ := mm.sm.LoadOrStore(key, &sync.RWMutex{})
	return actual.(*sync.RWMutex) //nolint:errcheck // Type assertion always succeeds.
}
