package data

import (
	"context"
	"strings"
	"sync"

	"go.temporal.io/sdk/workflow"
)

const (
	urlsIDsFile = "urls_ids.json"
)

var urlsIDsMutex sync.RWMutex

// MapURLAndID saves a 2-way mapping between a PR URL and its dedicated chat channel or thread IDs.
func MapURLAndID(ctx workflow.Context, url, id string) error {
	m, err := readURLsIDsFile(ctx)
	if err != nil {
		return err
	}

	m[url] = id
	m[id] = url

	urlsIDsMutex.Lock()
	defer urlsIDsMutex.Unlock()

	if ctx == nil { // For unit testing.
		return writeJSONActivity(context.Background(), urlsIDsFile, m)
	}
	return executeLocalActivity(ctx, writeJSONActivity, nil, urlsIDsFile, m)
}

// SwitchURLAndID converts a PR URL to its mapped chat channel or thread IDs, and vice versa.
func SwitchURLAndID(ctx workflow.Context, key string) (string, error) {
	m, err := readURLsIDsFile(ctx)
	if err != nil {
		return "", err
	}

	return m[key], nil
}

// DeleteURLAndIDMapping deletes the 2-way mapping between PR URLs and chat channel and thread
// IDs when they become obsolete (i.e. when the PR is merged, closed, or marked as a draft).
func DeleteURLAndIDMapping(ctx workflow.Context, key string) error {
	m, err := readURLsIDsFile(ctx)
	if err != nil {
		return err
	}

	delete(m, m[key])
	delete(m, key)

	prefix := key + "/"
	var moreKeysToDelete []string
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			moreKeysToDelete = append(moreKeysToDelete, k, m[k])
		}
	}
	for _, k := range moreKeysToDelete {
		delete(m, k)
	}

	urlsIDsMutex.Lock()
	defer urlsIDsMutex.Unlock()

	if ctx == nil { // For unit testing.
		return writeJSONActivity(context.Background(), urlsIDsFile, m)
	}
	return executeLocalActivity(ctx, writeJSONActivity, nil, urlsIDsFile, m)
}

func readURLsIDsFile(ctx workflow.Context) (map[string]string, error) {
	urlsIDsMutex.RLock()
	defer urlsIDsMutex.RUnlock()

	if ctx == nil { // For unit testing.
		return readJSONActivity(context.Background(), urlsIDsFile)
	}

	file := map[string]string{}
	if err := executeLocalActivity(ctx, readJSONActivity, &file, urlsIDsFile); err != nil {
		return nil, err
	}
	return file, nil
}
