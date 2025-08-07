package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

const (
	usersFileName = "users.json"

	bitbucketPrefix = "bitbucket"
	githubPrefix    = "github"
	slackPrefix     = "slack"
)

// AddBitbucketUser saves a 2-way mapping between
// a Bitbucket user's account ID and email address.
func AddBitbucketUser(accountID, email string) error {
	return addUser(bitbucketPrefix, accountID, email)
}

// AddBitbucketUser saves a 2-way mapping between
// a GitHub user's username and email address.
func AddGitHubUser(username, email string) error {
	return addUser(githubPrefix, username, email)
}

// AddBitbucketUser saves a 2-way mapping between
// a Slack user's user ID and email address.
func AddSlackUser(userID, email string) error {
	return addUser(slackPrefix, userID, email)
}

// BitbucketUserEmailByID returns a Bitbucket user's email based on
// their account ID. Returns an empty string if the user is not found.
func BitbucketUserEmailByID(accountID string) (string, error) {
	return userEmailByID(bitbucketPrefix, accountID)
}

// GitHubUserEmailByID returns a GitHub user's email based on their
// username. Returns an empty string if the user is not found.
func GitHubUserEmailByID(username string) (string, error) {
	return userEmailByID(githubPrefix, username)
}

// SlackUserEmailByID returns a Slack user's email based on their
// user ID. Returns an empty string if the user is not found.
func SlackUserEmailByID(userID string) (string, error) {
	return userEmailByID(slackPrefix, userID)
}

// BitbucketUserIDByEmail returns a Bitbucket user's account ID based
// on their email. Returns an empty string if the user is not found.
func BitbucketUserIDByEmail(email string) (string, error) {
	return userIDByEmail(bitbucketPrefix, email)
}

// GitHubUserIDByEmail returns a GitHub user's username based on
// their email. Returns an empty string if the user is not found.
func GitHubUserIDByEmail(email string) (string, error) {
	return userIDByEmail(githubPrefix, email)
}

// SlackUserIDByEmail returns a Slack user's user ID based on
// their email. Returns an empty string if the user is not found.
func SlackUserIDByEmail(email string) (string, error) {
	return userIDByEmail(slackPrefix, email)
}

// RemoveBitbucketUser forgets a Bitbucket user's account ID and email.
func RemoveBitbucketUser(email string) error {
	return removeUser(bitbucketPrefix, email)
}

// RemoveGitHubUser forgets a GitHub user's username and email.
func RemoveGitHubUser(email string) error {
	return removeUser(githubPrefix, email)
}

// RemoveSlackUser forgets a Slack user's user ID and email.
func RemoveSlackUser(email string) error {
	return removeUser(slackPrefix, email)
}

func addUser(prefix, id, email string) error {
	path := dataPath(usersFileName)

	m, err := readJSONMaps(path)
	if err != nil {
		return err
	}

	m["ids"][fmt.Sprintf("%s/%s", prefix, id)] = email
	m["emails"][fmt.Sprintf("%s/%s", prefix, email)] = id

	return writeSONMaps(path, m)
}

func userEmailByID(prefix, id string) (string, error) {
	path := dataPath(usersFileName)

	m, err := readJSONMaps(path)
	if err != nil {
		return "", err
	}

	return m["ids"][fmt.Sprintf("%s/%s", prefix, id)], nil
}

func userIDByEmail(prefix, email string) (string, error) {
	path := dataPath(usersFileName)

	m, err := readJSONMaps(path)
	if err != nil {
		return "", err
	}

	return m["email"][fmt.Sprintf("%s/%s", prefix, email)], nil
}

func removeUser(prefix, email string) error {
	path := dataPath(usersFileName)

	m, err := readJSONMaps(path)
	if err != nil {
		return err
	}

	email = fmt.Sprintf("%s/%s", prefix, email)
	id := fmt.Sprintf("%s/%s", prefix, m["emails"][email])
	delete(m["emails"], email)
	delete(m["ids"], id)

	return writeSONMaps(path, m)
}

func readJSONMaps(path string) (map[string]map[string]string, error) {
	f, err := os.ReadFile(path) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return nil, err
	}

	// Special case: empty files can't be parsed as JSON,
	// but they still represent a valid initial state.
	m := map[string]map[string]string{}
	if len(f) == 0 {
		return m, nil
	}

	if err := json.NewDecoder(bytes.NewReader(f)).Decode(&m); err != nil {
		return nil, err
	}

	return m, nil
}

func writeSONMaps(path string, m map[string]map[string]string) error {
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(m)
}
