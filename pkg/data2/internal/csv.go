package internal

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
)

const (
	slackChannelsFile = "slack_channels_log.csv"
)

func AppendToCSVFile(_ context.Context, record []string) error {
	mu := dataFileMutexes.Get(slackChannelsFile)
	mu.Lock()
	defer mu.Unlock()

	path, err := dataPath(slackChannelsFile)
	if err != nil {
		return fmt.Errorf("failed to get data file path: %w", err)
	}

	appendFlags := os.O_APPEND | os.O_CREATE | os.O_WRONLY // != [fileFlags] to avoid truncation.
	f, err := os.OpenFile(path, appendFlags, filePerms)    //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(record); err != nil {
		return fmt.Errorf("failed to append record to CSV file: %w", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV writer: %w", err)
	}

	return nil
}
