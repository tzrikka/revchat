package data

import (
	"context"
	"log/slog"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data/internal"
)

func LogSlackChannelArchived(ctx workflow.Context, channelID, prURL string) {
	appendToCSVFile(ctx, []string{now(ctx), "archived", channelID, prURL})
}

func LogSlackChannelCreated(ctx workflow.Context, channelID, prURL, name string) {
	appendToCSVFile(ctx, []string{now(ctx), "created", channelID, prURL, name})
}

func appendToCSVFile(ctx workflow.Context, record []string) {
	if ctx == nil { // For unit testing.
		_ = internal.AppendToCSVFile(context.Background(), record) //workflowcheck:ignore
		return
	}

	if err := executeLocalActivity(ctx, internal.AppendToCSVFile, nil, record); err != nil {
		logger.From(ctx).Error("failed to append record to Slack channels log", slog.Any("error", err),
			slog.String("event", record[1]), slog.String("channel_id", record[2]), slog.String("pr_url", record[3]))
	}
}

func now(ctx workflow.Context) string {
	if ctx == nil { // For unit testing.
		return time.Now().UTC().Format(time.RFC3339) //workflowcheck:ignore
	}
	return workflow.Now(ctx).UTC().Format(time.RFC3339)
}
