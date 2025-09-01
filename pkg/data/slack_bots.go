package data

const (
	slackBotsFile = "slack_bots.json"
)

func SetSlackBotUserID(botID, userID string) error {
	path := dataPath(slackBotsFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	m[botID] = userID
	return writeJSON(path, m)
}

func GetSlackBotUserID(botID string) (string, error) {
	m, err := readJSON(dataPath(slackBotsFile))
	if err != nil {
		return "", err
	}

	return m[botID], nil
}

func DeleteSlackBotIDs(botID string) error {
	path := dataPath(slackBotsFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	delete(m, botID)
	return writeJSON(path, m)
}
