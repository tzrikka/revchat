package data

const (
	slackBotsFile = "slack_bots.toml"
)

func SetSlackBotUserID(botID, userID string) error {
	path := dataPath(slackBotsFile)

	m, err := readTOML(path)
	if err != nil {
		return err
	}

	m[botID] = userID
	return writeTOML(path, m)
}

func DeleteSlackBotIDs(botID string) error {
	path := dataPath(slackBotsFile)

	m, err := readTOML(path)
	if err != nil {
		return err
	}

	delete(m, botID)
	return writeTOML(path, m)
}

func GetSlackBotUserID(botID string) (string, error) {
	m, err := readTOML(dataPath(slackBotsFile))
	if err != nil {
		return "", err
	}

	return m[botID], nil
}
