package data

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data/internal"
)

// MapURLAndID saves a 2-way mapping between PR and PR-comment URLs and their corresponding Slack channel and
// thread IDs. An error in mapping a new Slack channel is critical, but an error in mapping Slack messages isn't.
func MapURLAndID(ctx workflow.Context, url, ids string) error {
	if ctx == nil { // For unit testing.
		return internal.SetURLAndIDMapping(context.Background(), url, ids)
	}

	if err := executeLocalActivity(ctx, internal.SetURLAndIDMapping, nil, url, ids); err != nil {
		logger.From(ctx).Error("failed to set mapping between PR URLs and Slack IDs",
			slog.Any("error", err), slog.String("pr_url", url), slog.String("slack_ids", ids))
		return err
	}

	return nil
}

// SwitchURLAndID converts the URL of a PR or PR comment into the corresponding channel or thread IDs, and vice versa.
func SwitchURLAndID(ctx workflow.Context, key string) (string, error) {
	if ctx == nil { // For unit testing.
		return internal.GetURLAndIDMapping(context.Background(), key)
	}

	var val string
	if err := executeLocalActivity(ctx, internal.GetURLAndIDMapping, &val, key); err != nil {
		logger.From(ctx).Error("failed to get mapping between PR URLs and Slack IDs",
			slog.Any("error", err), slog.String("key", key))
		return "", err
	}

	if val == "" {
		workflow.SideEffect(ctx, func(_ workflow.Context) any { return []string{"URL/ID not found", key} })
	}
	return val, nil
}

// DeleteURLAndIDMapping deletes the 2-way mapping between PR and PR-comment URLs and their corresponding Slack channel and
// thread IDs when they become obsolete. Errors here are notable but not critical, so they are logged but not returned.
func DeleteURLAndIDMapping(ctx workflow.Context, key string) {
	if ctx == nil { // For unit testing.
		_ = internal.DelURLAndIDMapping(context.Background(), key)
		return
	}

	if err := executeLocalActivity(ctx, internal.DelURLAndIDMapping, nil, key); err != nil {
		logger.From(ctx).Error("failed to delete mapping between PR URLs and Slack IDs",
			slog.Any("error", err), slog.String("key", key))
	}
}
