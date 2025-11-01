package data

import (
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	optInFile = "opt_in.json"
)

func OptInBitbucketUser(slackUserID, accountID, email, linkID string) error {
	if err := AddSlackUser(slackUserID, email); err != nil {
		return err
	}

	if err := AddBitbucketUser(accountID, email); err != nil {
		return err
	}

	return optInUser(email, linkID)
}

func OptInGitHubUser(slackUserID, username, email, linkID string) error {
	if err := AddSlackUser(slackUserID, email); err != nil {
		return err
	}

	if err := AddGitHubUser(username, email); err != nil {
		return err
	}

	return optInUser(email, linkID)
}

func optInUser(email, linkID string) error {
	path := dataPath(optInFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	m[email] = linkID
	return writeJSON(path, m)
}

func IsOptedIn(email string) (bool, error) {
	linkID, err := UserLinkID(email)
	if err != nil {
		return false, err
	}

	return linkID != "", nil
}

func UserLinkID(email string) (string, error) {
	if email == "" || email == "bot" {
		return "", nil
	}

	m, err := readJSON(dataPath(optInFile))
	if err != nil {
		return "", err
	}

	return m[email], nil
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

// dataPath returns the absolute path to the given data file.
// It also creates an empty file if it doesn't already exist.
func dataPath(filename string) string {
	path, _ := xdg.CreateFile(xdg.DataHome, config.DirName, filename)
	return path
}
