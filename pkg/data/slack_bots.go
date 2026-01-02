package data

import (
	"sync"

	"go.temporal.io/sdk/workflow"
)

const (
	slackBotsFile = "slack_bots.json"
)

var slackBotsMutex sync.RWMutex

func SetSlackBotUserID(ctx workflow.Context, botID, userID string) error {
	m, err := readSlackBotsFile(ctx)
	if err != nil {
		return err
	}

	m[botID] = userID

	slackBotsMutex.Lock()
	defer slackBotsMutex.Unlock()

	return executeLocalActivity(ctx, writeJSONActivity, nil, slackBotsFile, m)
}

func DeleteSlackBotIDs(ctx workflow.Context, botID string) error {
	m, err := readSlackBotsFile(ctx)
	if err != nil {
		return err
	}

	delete(m, botID)

	slackBotsMutex.Lock()
	defer slackBotsMutex.Unlock()

	return executeLocalActivity(ctx, writeJSONActivity, nil, slackBotsFile, m)
}

func GetSlackBotUserID(ctx workflow.Context, botID string) (string, error) {
	m, err := readSlackBotsFile(ctx)
	if err != nil {
		return "", err
	}

	return m[botID], nil
}

// readSlackBotsFile is a thin wrapper over [readJSONActivity].
func readSlackBotsFile(ctx workflow.Context) (map[string]string, error) {
	slackBotsMutex.RLock()
	defer slackBotsMutex.RUnlock()

	file := map[string]string{}
	if err := executeLocalActivity(ctx, readJSONActivity, &file, slackBotsFile); err != nil {
		return nil, err
	}

	return file, nil
}
