package data

import "sync"

const (
	slackBotsFile = "slack_bots.json"
)

func SetSlackBotUserID(botID, userID string) error {
	m, err := readSlackBotsFile()
	if err != nil {
		return err
	}

	m[botID] = userID
	return writeSlackBotsFile(m)
}

func DeleteSlackBotIDs(botID string) error {
	m, err := readSlackBotsFile()
	if err != nil {
		return err
	}

	delete(m, botID)
	return writeSlackBotsFile(m)
}

func GetSlackBotUserID(botID string) (string, error) {
	m, err := readSlackBotsFile()
	if err != nil {
		return "", err
	}

	return m[botID], nil
}

var slackBotsMutex sync.RWMutex

func readSlackBotsFile() (map[string]string, error) {
	slackBotsMutex.RLock()
	defer slackBotsMutex.RUnlock()

	return readJSON(slackBotsFile)
}

func writeSlackBotsFile(m map[string]string) error {
	slackBotsMutex.Lock()
	defer slackBotsMutex.Unlock()

	return writeJSON(slackBotsFile, m)
}
