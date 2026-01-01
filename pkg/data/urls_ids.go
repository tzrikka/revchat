package data

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

const (
	urlsIDsFile = "urls_ids.json"
)

var urlsIDsMutex sync.RWMutex

// MapURLAndID saves a 2-way mapping between PR and PR comment URLs and their corresponding Slack channel and
// thread IDs. An error in mapping a new Slack channel is critical, but an error in mapping Slack messages isn't.
func MapURLAndID(ctx workflow.Context, url, id string) error {
	m, err := readURLsIDsFile(ctx)
	if err != nil {
		return err
	}

	m[url] = id
	m[id] = url

	return writeURLsIDsFile(ctx, m)
}

// SwitchURLAndID converts the URL of a PR or PR comment into the corresponding channel or thread IDs, and vice versa.
func SwitchURLAndID(ctx workflow.Context, key string) (string, error) {
	m, err := readURLsIDsFile(ctx)
	if err != nil {
		return "", err
	}

	return m[key], nil
}

// DeleteURLAndIDMapping deletes the 2-way mapping between PR and PR comment URLs and their corresponding Slack channel and
// thread IDs when they become obsolete. Errors here are notable but not critical, so they are logged but not returned.
func DeleteURLAndIDMapping(ctx workflow.Context, key string) {
	m, err := readURLsIDsFile(ctx)
	if err != nil {
		return
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
		_ = writeJSONActivity(context.Background(), urlsIDsFile, m)
	}
	_ = executeLocalActivity(ctx, writeJSONActivity, nil, urlsIDsFile, m)
}

func readURLsIDsFile(ctx workflow.Context) (map[string]string, error) {
	urlsIDsMutex.RLock()
	defer urlsIDsMutex.RUnlock()

	if ctx == nil { // For unit testing.
		return readJSONActivity(context.Background(), urlsIDsFile)
	}

	file := map[string]string{}
	if err := executeLocalActivity(ctx, readJSONActivity, &file, urlsIDsFile); err != nil {
		logger.From(ctx).Error("failed to read mapping of PR URLs and Slack IDs", slog.Any("error", err))
		return nil, err
	}

	return file, nil
}

func writeURLsIDsFile(ctx workflow.Context, m map[string]string) error {
	urlsIDsMutex.Lock()
	defer urlsIDsMutex.Unlock()

	if ctx == nil { // For unit testing.
		return writeJSONActivity(context.Background(), urlsIDsFile, m)
	}

	err := executeLocalActivity(ctx, writeJSONActivity, nil, urlsIDsFile, m)
	if err != nil {
		logger.From(ctx).Error("failed to write mapping of PR URLs and Slack IDs", slog.Any("error", err))
		return err
	}

	return nil
}
