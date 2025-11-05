package data

import (
	"sync"
	"time"

	"github.com/tzrikka/revchat/pkg/metrics"
)

const (
	slackChannelsFile = "slack_channels_log.csv"
)

var slackChannelsMutex sync.Mutex

func LogSlackChannelArchived(channelID, url string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	now, path, err := timestampAndPath()
	if err != nil {
		return err
	}

	return metrics.AppendToCSVFile(path, []string{now, "archived", channelID, url})
}

func LogSlackChannelCreated(channelID, name, url string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	now, path, err := timestampAndPath()
	if err != nil {
		return err
	}

	return metrics.AppendToCSVFile(path, []string{now, "created", channelID, name, url})
}

func LogSlackChannelDeleted(channelID string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	now, path, err := timestampAndPath()
	if err != nil {
		return err
	}

	return metrics.AppendToCSVFile(path, []string{now, "deleted", channelID})
}

func LogSlackChannelRenamed(channelID, name string) error {
	slackChannelsMutex.Lock()
	defer slackChannelsMutex.Unlock()

	now, path, err := timestampAndPath()
	if err != nil {
		return err
	}

	return metrics.AppendToCSVFile(path, []string{now, "renamed", channelID, name})
}

func timestampAndPath() (now, path string, err error) {
	now = time.Now().UTC().Format(time.RFC3339)
	path, err = cachedDataPath(slackChannelsFile, "")
	return
}
