package internal

import (
	"context"
)

const (
	slackBotsFile = "slack_bots.json"
)

func SetSlackBotUserID(_ context.Context, botID, userID string) error {
	mu := getDataFileMutex(slackBotsFile)
	mu.Lock()
	defer mu.Unlock()

	m, err := readGenericJSONFile(slackBotsFile)
	if err != nil {
		return err
	}

	m[botID] = userID
	return writeGenericJSONFile(slackBotsFile, m)
}

func GetSlackBotUserID(_ context.Context, botID string) (string, error) {
	mu := getDataFileMutex(slackBotsFile)
	mu.Lock()
	defer mu.Unlock()

	m, err := readGenericJSONFile(slackBotsFile)
	if err != nil {
		return "", err
	}

	return m[botID], nil
}
