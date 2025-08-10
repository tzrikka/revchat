package data

import (
	"time"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	optInFile = "opt_in.json"
)

// dataPath returns the absolute path to the given data file.
// It also creates an empty file if it doesn't already exist.
func dataPath(filename string) string {
	path, _ := xdg.CreateFile(xdg.DataHome, config.DirName, filename)
	return path
}

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
