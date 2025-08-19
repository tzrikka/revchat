package data

import (
	"time"
)

const (
	optInFile = "opt_in.json"
)

func OptInBitbucketUser(slackUserID, accountID, email string) error {
	if err := AddSlackUser(slackUserID, email); err != nil {
		return err
	}

	if err := AddBitbucketUser(accountID, email); err != nil {
		return err
	}

	return optInUser(email)
}

func OptInGitHubUser(slackUserID, username, email string) error {
	if err := AddSlackUser(slackUserID, email); err != nil {
		return err
	}

	if err := AddGitHubUser(username, email); err != nil {
		return err
	}

	return optInUser(email)
}

func optInUser(email string) error {
	path := dataPath(optInFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	if m[email] == "" {
		m[email] = time.Now().UTC().Format(time.RFC3339)
	}

	return writeJSON(path, m)
}

func IsOptedIn(email string) (bool, error) {
	if email == "" || email == "bot" {
		return false, nil
	}

	m, err := readJSON(dataPath(optInFile))
	if err != nil {
		return false, err
	}

	return m[email] != "", nil
}

func OptOut(email string) error {
	path := dataPath(optInFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	delete(m, email)
	return writeJSON(path, m)
}
