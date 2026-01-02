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
func MapURLAndID(ctx workflow.Context, url, ids string) error {
	urlsIDsMutex.Lock()
	defer urlsIDsMutex.Unlock()

	if ctx == nil { // For unit testing.
		return setMapEntryActivity(context.Background(), url, ids)
	}

	if err := executeLocalActivity(ctx, setMapEntryActivity, nil, url, ids); err != nil {
		logger.From(ctx).Error("failed to set mapping between PR URLs and Slack IDs",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("slack_ids", ids))
		return err
	}

	return nil
}

// SwitchURLAndID converts the URL of a PR or PR comment into the corresponding channel or thread IDs, and vice versa.
func SwitchURLAndID(ctx workflow.Context, key string) (string, error) {
	urlsIDsMutex.RLock()
	defer urlsIDsMutex.RUnlock()

	if ctx == nil { // For unit testing.
		return getMapEntryActivity(context.Background(), key)
	}

	var val string
	if err := executeLocalActivity(ctx, getMapEntryActivity, &val, key); err != nil {
		logger.From(ctx).Warn("failed to get mapping between PR URLs and Slack IDs",
			slog.Any("error", err), slog.String("key", key))
		return "", err
	}

	return val, nil
}

// DeleteURLAndIDMapping deletes the 2-way mapping between PR and PR comment URLs and their corresponding Slack channel and
// thread IDs when they become obsolete. Errors here are notable but not critical, so they are logged but not returned.
func DeleteURLAndIDMapping(ctx workflow.Context, key string) {
	urlsIDsMutex.Lock()
	defer urlsIDsMutex.Unlock()

	if ctx == nil { // For unit testing.
		_ = delMapEntryActivity(context.Background(), key)
		return
	}

	if err := executeLocalActivity(ctx, delMapEntryActivity, nil, key); err != nil {
		logger.From(ctx).Error("failed to delete mapping between PR URLs and Slack IDs",
			slog.Any("error", err), slog.String("key", key))
		return
	}
}

func setMapEntryActivity(ctx context.Context, url, ids string) error {
	m, err := readJSONActivity(ctx, urlsIDsFile)
	if err != nil {
		return err
	}

	m[url] = ids
	m[ids] = url

	return writeJSONActivity(ctx, urlsIDsFile, m)
}

func getMapEntryActivity(ctx context.Context, key string) (string, error) {
	m, err := readJSONActivity(ctx, urlsIDsFile)
	if err != nil {
		return "", err
	}

	return m[key], nil
}

func delMapEntryActivity(ctx context.Context, key string) error {
	m, err := readJSONActivity(ctx, urlsIDsFile)
	if err != nil {
		return err
	}

	if v, ok := m[key]; ok {
		delete(m, v)
	}
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

	return writeJSONActivity(ctx, urlsIDsFile, m)
}
