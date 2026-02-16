package data

import (
	"encoding/csv"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

const (
	slackChannelsFile = "slack_channels_log.csv"
)

var slackChannelsMutex sync.Mutex

func LogSlackChannelArchived(ctx workflow.Context, channelID, url string) {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	appendToCSVFile(ctx, []string{"archived", channelID, url})
}

func LogSlackChannelCreated(ctx workflow.Context, channelID, name, url string) {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	appendToCSVFile(ctx, []string{"created", channelID, name, url})
}

func appendToCSVFile(ctx workflow.Context, record []string) {
	path, err := cachedDataPath(slackChannelsFile)
	if err != nil {
		logger.From(ctx).Error("failed to find/create Slack channels log file",
			slog.Any("error", err), slog.String("filename", slackChannelsFile))
	}

	now := workflow.Now(ctx).UTC().Format(time.RFC3339)
	record = append([]string{now}, record...)

	workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		appendFlags := os.O_APPEND | os.O_CREATE | os.O_WRONLY // != [fileFlags] to avoid truncation.
		f, err := os.OpenFile(path, appendFlags, filePerms)    //gosec:disable G304 // Hardcoded path.
		if err != nil {
			logger.From(ctx).Error("failed to open Slack channels log file",
				slog.Any("error", err), slog.Any("record", record))
			return nil
		}
		defer f.Close()

		w := csv.NewWriter(f)
		if err := w.Write(record); err != nil {
			logger.From(ctx).Error("failed to append to Slack channels log file",
				slog.Any("error", err), slog.Any("record", record))
			return nil
		}

		w.Flush()
		if err := w.Error(); err != nil {
			logger.From(ctx).Error("failed to flush Slack channels log file",
				slog.Any("error", err), slog.Any("record", record))
		}

		return nil
	})
}
