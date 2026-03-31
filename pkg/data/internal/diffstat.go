package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

const (
	DiffstatFileSuffix = "_diffstat.json"
)

func WriteDiffstat(_ context.Context, prURL string, files any) error {
	mu := getDataFileMutex(prURL + DiffstatFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	return writeGenericJSONFile(prURL+DiffstatFileSuffix, files)
}

func ReadDiffstatPaths(_ context.Context, prURL string) ([]string, error) {
	mu := getDataFileMutex(prURL + DiffstatFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	files, err := readDiffstat(prURL)
	if err != nil || len(files) == 0 {
		return nil, err
	}

	return extractFilePaths(files), nil
}

// readDiffstat expects the calling function to hold the appropriate mutex for the given PR URL.
func readDiffstat(prURL string) ([]map[string]any, error) {
	path, err := dataPath(prURL + DiffstatFileSuffix)
	if err != nil {
		return nil, fmt.Errorf("failed to get file path: %w", err)
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	ds := []map[string]any{}
	if err := json.NewDecoder(f).Decode(&ds); err != nil {
		return nil, fmt.Errorf("failed to read/decode JSON: %w", err)
	}
	return ds, nil
}

func extractFilePaths(files []map[string]any) []string {
	if paths := extractFilePathsBitbucket(files); len(paths) > 0 {
		return paths
	}
	return extractFilePathsGitHub(files)
}

func extractFilePathsBitbucket(files []map[string]any) []string {
	var paths []string
	for _, diffstat := range files {
		for _, key := range []string{"new", "old"} {
			block, ok := diffstat[key].(map[string]any)
			if !ok {
				continue
			}
			path, ok := block["path"].(string)
			if !ok {
				continue
			}
			paths = append(paths, path)
		}
	}

	slices.Sort(paths)
	return slices.Compact(paths)
}

func extractFilePathsGitHub(files []map[string]any) []string {
	paths := make([]string, 0, len(files))
	for _, diffstat := range files {
		path, ok := diffstat["filename"].(string)
		if !ok {
			continue
		}
		paths = append(paths, path)
	}

	if len(paths) == 0 {
		return nil
	}

	slices.Sort(paths)
	return slices.Compact(paths)
}
