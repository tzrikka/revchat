package data

import (
	"encoding/json"
	"os"
	"slices"

	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

var prDiffstatMutexes RWMutexMap

func ReadBitbucketDiffstatLength(url string) int {
	return len(diffstatPaths(readBitbucketDiffstat(url)))
}

func ReadBitbucketDiffstatPaths(url string) []string {
	return diffstatPaths(readBitbucketDiffstat(url))
}

func readBitbucketDiffstat(url string) []bitbucket.Diffstat {
	mu := prDiffstatMutexes.Get(url)
	mu.RLock()
	defer mu.RUnlock()

	path, err := cachedDataPath(url, "_diffstat")
	if err != nil {
		return nil
	}

	f, err := os.Open(path) //gosec:disable G304 -- URL received from signature-verified 3rd-party
	if err != nil {
		return nil
	}
	defer f.Close()

	ds := []bitbucket.Diffstat{}
	if err := json.NewDecoder(f).Decode(&ds); err != nil {
		return nil
	}

	return ds
}

func diffstatPaths(ds []bitbucket.Diffstat) []string {
	var paths []string
	for _, d := range ds {
		if d.New != nil {
			paths = append(paths, d.New.Path)
		}
		if d.Old != nil {
			paths = append(paths, d.Old.Path)
		}
	}

	slices.Sort(paths)
	return slices.Compact(paths)
}

func UpdateBitbucketDiffstat(url string, ds []bitbucket.Diffstat) error {
	mu := prDiffstatMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	path, err := cachedDataPath(url, "_diffstat")
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
	return e.Encode(ds)
}

func DeleteBitbucketDiffstat(url string) error {
	mu := prDiffstatMutexes.Get(url)
	mu.Lock()
	defer mu.Unlock()

	path, err := cachedDataPath(url, "_diffstat")
	if err != nil {
		return err
	}

	return os.Remove(path) //gosec:disable G304 -- URL received from signature-verified 3rd-party
}
