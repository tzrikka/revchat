package data

import (
	"log/slog"
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/metrics"
)

const (
	slackChannelsFile = "slack_channels_log.csv"
)

var slackChannelsMutex sync.Mutex

func LogSlackChannelArchived(ctx workflow.Context, channelID, url string) {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	if now, path, err := timestampAndPath(ctx); err == nil {
		appendToCSVFile(ctx, path, []string{now, "archived", channelID, url})
	}
}

func LogSlackChannelCreated(ctx workflow.Context, channelID, name, url string) {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	if now, path, err := timestampAndPath(ctx); err == nil {
		appendToCSVFile(ctx, path, []string{now, "created", channelID, name, url})
	}
}

func LogSlackChannelRenamed(ctx workflow.Context, channelID, name string) {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	if now, path, err := timestampAndPath(ctx); err == nil {
		appendToCSVFile(ctx, path, []string{now, "renamed", channelID, name})
	}
}

func timestampAndPath(ctx workflow.Context) (now, path string, err error) {
	path, err = cachedDataPath(slackChannelsFile, "")
	if err != nil {
		logger.From(ctx).Error("failed to find/create Slack channel log file",
			slog.Any("error", err), slog.String("filename", slackChannelsFile))
	}
	return time.Now().UTC().Format(time.RFC3339), path, err
}

func appendToCSVFile(ctx workflow.Context, path string, record []string) {
	if err := metrics.AppendToCSVFile(path, record); err != nil {
		logger.From(ctx).Error("failed to append to Slack channels log file",
			slog.Any("error", err), slog.Any("record", record))
	}
}
