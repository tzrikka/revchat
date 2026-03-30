package internal

import (
	"context"
	"strings"
)

const (
	urlsIDsFile = "urls_ids.json"
)

// GetURLAndIDMapping expects the caller to hold the appropriate mutex.
func GetURLAndIDMapping(_ context.Context, key string) (string, error) {
	mu := dataFileMutexes.Get(urlsIDsFile)
	mu.Lock()
	defer mu.Unlock()

	m, err := readGenericJSONFile(urlsIDsFile)
	if err != nil {
		return "", err
	}

	return m[key], nil
}

// SetURLAndIDMapping expects the caller to hold the appropriate mutex.
func SetURLAndIDMapping(_ context.Context, url, ids string) error {
	mu := dataFileMutexes.Get(urlsIDsFile)
	mu.Lock()
	defer mu.Unlock()

	m, err := readGenericJSONFile(urlsIDsFile)
	if err != nil {
		return err
	}

	m[url] = ids
	m[ids] = url

	return writeGenericJSONFile(urlsIDsFile, m)
}

// DelURLAndIDMapping expects the caller to hold the appropriate mutex.
func DelURLAndIDMapping(_ context.Context, key string) error {
	mu := dataFileMutexes.Get(urlsIDsFile)
	mu.Lock()
	defer mu.Unlock()

	m, err := readGenericJSONFile(urlsIDsFile)
	if err != nil {
		return err
	}

	if v, ok := m[key]; ok {
		delete(m, v)
	}
	delete(m, key)

	for k, v := range m {
		if v == key {
			delete(m, k)
		}
	}

	prefix := key + "/"
	var moreKeysToDelete []string
	for k, v := range m {
		if strings.HasPrefix(k, prefix) {
			moreKeysToDelete = append(moreKeysToDelete, k, v)
		}
		if strings.HasPrefix(v, prefix) {
			moreKeysToDelete = append(moreKeysToDelete, k, v)
		}
	}
	for _, k := range moreKeysToDelete {
		delete(m, k)
	}

	return writeGenericJSONFile(urlsIDsFile, m)
}
