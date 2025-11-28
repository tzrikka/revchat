package data

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

const (
	usersFile = "users.json"

	indexByEmail       = 1
	indexByBitbucketID = 2
	indexByGitHubID    = 3
	indexBySlackID     = 4
)

// User represents a mapping between various user IDs and email
// addresses across different platforms (Bitbucket, GitHub, Slack).
type User struct {
	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
	Deleted string `json:"deleted,omitempty"`

	Email       string `json:"email,omitempty"`
	BitbucketID string `json:"bitbucket_id,omitempty"`
	GitHubID    string `json:"github_id,omitempty"`
	SlackID     string `json:"slack_id,omitempty"`

	RealName  string `json:"real_name,omitempty"`
	SlackName string `json:"slack_name,omitempty"`

	ThrippyLink string `json:"thrippy_link,omitempty"`
}

// Users is an indexed copy of a collection of [User] entries.
// This should really be stored in a relational database.
type Users struct {
	entries []User

	emailIndex     map[string]int
	bitbucketIndex map[string]int
	githubIndex    map[string]int
	slackIndex     map[string]int
}

var (
	usersDB    *Users
	usersMutex sync.Mutex
)

func UpsertUser(email, bitbucketID, githubID, slackID, thrippyLink string) error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	if usersDB == nil {
		var err error
		usersDB, err = readUsersFile()
		if err != nil {
			return err
		}
	}

	i, err := usersDB.findUserIndex(email, bitbucketID, githubID, slackID)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if i == -1 {
		i = len(usersDB.entries)
		usersDB.entries = append(usersDB.entries, User{Created: now}) // Insert a new empty entry.
	}
	usersDB.entries[i].Updated = now

	// Allow partial updates, don't overwrite existing fields with empty values.
	// (we already checked for conflicts in [Users.findIndex]).
	if email != "" {
		usersDB.entries[i].Email = email
		if email != "bot" {
			usersDB.emailIndex[email] = i
		}
	}
	if bitbucketID != "" {
		usersDB.entries[i].BitbucketID = bitbucketID
		usersDB.bitbucketIndex[bitbucketID] = i
	}
	if githubID != "" {
		usersDB.entries[i].GitHubID = githubID
		usersDB.githubIndex[githubID] = i
	}
	if slackID != "" {
		usersDB.entries[i].SlackID = slackID
		usersDB.slackIndex[slackID] = i
	}
	if thrippyLink != "" {
		if thrippyLink == "X" {
			thrippyLink = ""
		}
		usersDB.entries[i].ThrippyLink = thrippyLink
	}

	return usersDB.writeUsersFile()
}

func (u *Users) findUserIndex(email, bitbucketID, githubID, slackID string) (int, error) {
	emailIndex, emailFound := u.emailIndex[email]
	bitbucketIndex, bitbucketFound := u.bitbucketIndex[bitbucketID]
	githubIndex, githubFound := u.githubIndex[githubID]
	slackIndex, slackFound := u.slackIndex[slackID]

	i := -1
	if emailFound {
		i = emailIndex
	}
	if bitbucketFound {
		if i >= 0 && i != bitbucketIndex {
			return -1, errors.New("conflicting entries")
		}
		i = bitbucketIndex
	}
	if githubFound {
		if i >= 0 && i != githubIndex {
			return -1, errors.New("conflicting entries")
		}
		i = githubIndex
	}
	if slackFound {
		if i >= 0 && i != slackIndex {
			return -1, errors.New("conflicting entries")
		}
		i = slackIndex
	}

	return i, nil
}

func SelectUserByEmail(email string) (User, error) {
	return selectUserBy(indexByEmail, email)
}

func SelectUserByBitbucketID(bitbucketID string) (User, error) {
	return selectUserBy(indexByBitbucketID, bitbucketID)
}

func SelectUserByGitHubID(githubID string) (User, error) {
	return selectUserBy(indexByGitHubID, githubID)
}

func SelectUserBySlackID(slackID string) (User, error) {
	return selectUserBy(indexBySlackID, slackID)
}

func IsOptedIn(u User) bool {
	return u.ThrippyLink != ""
}

func selectUserBy(indexType int, id string) (User, error) {
	if id == "" {
		return User{}, nil
	}

	usersMutex.Lock()
	defer usersMutex.Unlock()

	if usersDB == nil {
		var err error
		usersDB, err = readUsersFile()
		if err != nil {
			return User{}, err
		}
	}

	var index map[string]int
	switch indexType {
	case indexByEmail:
		index = usersDB.emailIndex
	case indexByBitbucketID:
		index = usersDB.bitbucketIndex
	case indexByGitHubID:
		index = usersDB.githubIndex
	case indexBySlackID:
		index = usersDB.slackIndex
	default:
		return User{}, errors.New("invalid index type")
	}

	i, found := index[id]
	if !found {
		return User{}, nil
	}

	entryCopy := usersDB.entries[i]
	return entryCopy, nil
}

func readUsersFile() (*Users, error) {
	path, err := cachedDataPath(usersFile, "")
	if err != nil {
		return nil, err
	}

	// Special case: empty files can't be parsed as JSON, but this initial state is valid.
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	u := &Users{entries: []User{}}
	if fi.Size() > 0 {
		f, err := os.Open(path) //gosec:disable G304 -- specified by admin by design
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if err := json.NewDecoder(f).Decode(&u.entries); err != nil {
			return nil, err
		}
	}

	// Build indexes for fast lookups.
	u.emailIndex = map[string]int{}
	u.bitbucketIndex = map[string]int{}
	u.githubIndex = map[string]int{}
	u.slackIndex = map[string]int{}

	for i, user := range u.entries {
		if user.Email != "" && user.Email != "bot" {
			u.emailIndex[user.Email] = i
		}
		if user.BitbucketID != "" {
			u.bitbucketIndex[user.BitbucketID] = i
		}
		if user.GitHubID != "" {
			u.githubIndex[user.GitHubID] = i
		}
		if user.SlackID != "" {
			u.slackIndex[user.SlackID] = i
		}
	}

	return u, nil
}

func (u *Users) writeUsersFile() error {
	path, err := cachedDataPath(usersFile, "")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 -- specified by admin by design
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(u.entries)
}
