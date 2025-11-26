package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	fileFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	filePerms = xdg.NewFilePermissions
)

var pathCache = map[string]string{}

func readJSON(filename string) (map[string]string, error) {
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

	f, err := os.Open(path) //gosec:disable G304 -- specified by admin by design
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

func writeJSON(filename string, m map[string]string) error {
	path, err := cachedDataPath(filename, "")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 -- specified by admin by design
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(m)
}

func cachedDataPath(filename, suffix string) (string, error) {
	path, found := pathCache[filename+suffix]
	if found {
		return path, nil
	}

	// Special handling for PR diffstat/status/turn files.
	if strings.HasPrefix(filename, "https://") {
		path := urlBasedPath(filename, suffix)
		pathCache[filename+suffix] = path
		return path, nil
	}

	// Sanitize the filename, create the directory and the file if needed.
	path, err := xdg.CreateFile(xdg.DataHome, config.DirName, filename)
	if err != nil {
		return "", err
	}

	pathCache[filename+suffix] = path
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
	return actual.(*sync.RWMutex)
}
