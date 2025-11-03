package data

import (
	"encoding/csv"
	"os"
	"sync"
	"time"
)

const (
	slackChannelsFile = "slack_channels_log.csv"
)

var slackChannelsMutex sync.Mutex

func LogSlackChannelArchived(channelID, url string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	return appendToCSVFile([]string{"archived", channelID, url})
}

func LogSlackChannelCreated(channelID, name, url string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	return appendToCSVFile([]string{"created", channelID, name, url})
}

func LogSlackChannelDeleted(channelID string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	return appendToCSVFile([]string{"deleted", channelID})
}

func LogSlackChannelRenamed(channelID, name string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	return appendToCSVFile([]string{"renamed", channelID, name})
}

func appendToCSVFile(record []string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	path, err := cachedDataPath(slackChannelsFile)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerms) //gosec:disable G304 -- specified by admin by design
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(append([]string{now}, record...)); err != nil {
		return err
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return nil
}
