package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	usersFile = "users.json"

	IndexByEmail       = 1
	IndexByBitbucketID = 2
	IndexByGitHubID    = 3
	IndexBySlackID     = 4
	IndexByRealName    = 5
)

// User represents a mapping between a unique email address and
// at least 2 other unique identifiers for that person across
// different systems (Bitbucket, GitHub, Slack, Thrippy).
type User struct {
	Email string `json:"email,omitempty"`

	BitbucketID string `json:"bitbucket_id,omitempty"`
	GitHubID    string `json:"github_id,omitempty"`
	SlackID     string `json:"slack_id,omitempty"`
	ThrippyLink string `json:"thrippy_link,omitempty"`

	RealName string `json:"real_name,omitempty"` // Not guaranteed to be unique, unlike the fields above.

	// Slack user IDs, controlled by the un/follow Slack commands, used when creating channels.
	Followers []string `json:"followers,omitempty"`

	Created time.Time `json:"created,omitzero"`
	Updated time.Time `json:"updated,omitzero"`
	Deleted time.Time `json:"deleted,omitzero"`
}

func (u User) IsOptedIn() bool {
	return u.ThrippyLink != ""
}

// Users is an indexed copy of a collection of [User] entries.
// This should really be stored in a relational database.
type Users struct {
	entries []User

	emailIndex     map[string]int
	bitbucketIndex map[string]int
	githubIndex    map[string]int
	slackIndex     map[string]int
	nameIndex      map[string]int
}

var usersDB *Users

func initUsersDBIfNeeded() error {
	if usersDB == nil {
		var err error
		usersDB, err = readUsersFile()
		if err != nil {
			return err
		}
	}
	return nil
}

func UpsertUser(_ context.Context, email, realName, bitbucketID, githubID, slackID, thrippyLink string) (User, error) {
	mu := getDataFileMutex(usersFile)
	mu.Lock()
	defer mu.Unlock()

	if err := initUsersDBIfNeeded(); err != nil {
		return User{}, err
	}

	i, err := usersDB.findUserIndex(email, realName, bitbucketID, githubID, slackID)
	if err != nil {
		return User{}, err
	}

	now := time.Now().UTC()
	if i == -1 {
		i = len(usersDB.entries)
		usersDB.entries = append(usersDB.entries, User{Created: now}) // Insert a new empty entry.
	}
	usersDB.entries[i].Updated = now

	// Allow partial updates, but don't overwrite existing fields with empty values
	// (we already checked for conflicts in [Users.findUserIndex]).
	if email != "" {
		usersDB.entries[i].Email = email
		if email != "bot" {
			usersDB.emailIndex[email] = i
		}
	}
	if realName != "" {
		usersDB.entries[i].RealName = realName
		usersDB.nameIndex[realName] = i
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
		if thrippyLink == "X" { // Special value meaning "opt out".
			thrippyLink = ""
		}
		usersDB.entries[i].ThrippyLink = thrippyLink
	}

	user := usersDB.entries[i]
	return user, usersDB.writeUsersFile()
}

func FollowUser(_ context.Context, followerSlackID, followedSlackID string) (User, error) {
	mu := getDataFileMutex(usersFile)
	mu.Lock()
	defer mu.Unlock()

	if err := initUsersDBIfNeeded(); err != nil {
		return User{}, err
	}

	i, err := usersDB.findUserIndex("", "", "", "", followedSlackID)
	if err != nil || i < 0 {
		return User{}, err
	}

	done := false
	if !slices.Contains(usersDB.entries[i].Followers, followerSlackID) {
		usersDB.entries[i].Updated = time.Now().UTC()
		usersDB.entries[i].Followers = append(usersDB.entries[i].Followers, followerSlackID)
		slices.Sort(usersDB.entries[i].Followers)
		done = true
	}

	user := usersDB.entries[i]
	if !done {
		return user, nil
	}
	return user, usersDB.writeUsersFile()
}

func UnfollowUser(_ context.Context, followerSlackID, followedSlackID string) (User, error) {
	mu := getDataFileMutex(usersFile)
	mu.Lock()
	defer mu.Unlock()

	if err := initUsersDBIfNeeded(); err != nil {
		return User{}, err
	}

	user, err := unfollowUserWithoutLock(followerSlackID, followedSlackID)
	if err != nil {
		return User{}, err
	}

	if user.Updated.IsZero() {
		return user, nil
	}
	return user, usersDB.writeUsersFile()
}

// unfollowUserWithoutLock is the core logic of [UnfollowUser], extracted
// into a separate function to avoid deadlocks and extra writing when called by
// [RemoveFollower]. This function expects the caller to hold the appropriate mutex.
func unfollowUserWithoutLock(followerSlackID, followedSlackID string) (User, error) {
	i, err := usersDB.findUserIndex("", "", "", "", followedSlackID)
	if err != nil || i < 0 {
		return User{}, err
	}

	j := slices.Index(usersDB.entries[i].Followers, followerSlackID)
	if j < 0 {
		return User{}, nil
	}

	usersDB.entries[i].Followers = slices.Delete(usersDB.entries[i].Followers, j, j+1)
	usersDB.entries[i].Updated = time.Now().UTC()

	user := usersDB.entries[i]
	return user, nil
}

func RemoveFollower(_ context.Context, followerSlackID string) error {
	mu := getDataFileMutex(usersFile)
	mu.Lock()
	defer mu.Unlock()

	if err := initUsersDBIfNeeded(); err != nil {
		return err
	}

	done := false
	for _, user := range usersDB.entries {
		if slices.Contains(user.Followers, followerSlackID) {
			if _, err := unfollowUserWithoutLock(followerSlackID, user.SlackID); err != nil {
				return err
			}
			done = true
		}
	}

	if !done {
		return nil
	}
	return usersDB.writeUsersFile()
}

func (u *Users) findUserIndex(email, realName, bitbucketID, githubID, slackID string) (int, error) {
	emailIndex, emailFound := u.emailIndex[email]
	nameIndex, nameFound := u.nameIndex[realName]
	bitbucketIndex, bitbucketFound := u.bitbucketIndex[bitbucketID]
	githubIndex, githubFound := u.githubIndex[githubID]
	slackIndex, slackFound := u.slackIndex[slackID]

	i := -1
	if emailFound {
		i = emailIndex
	}
	if nameFound {
		if i >= 0 && i != nameIndex {
			return -1, errors.New("conflicting user entries")
		}
		i = nameIndex
	}
	if bitbucketFound {
		if i >= 0 && i != bitbucketIndex {
			return -1, errors.New("conflicting user entries")
		}
		i = bitbucketIndex
	}
	if githubFound {
		if i >= 0 && i != githubIndex {
			return -1, errors.New("conflicting user entries")
		}
		i = githubIndex
	}
	if slackFound {
		if i >= 0 && i != slackIndex {
			return -1, errors.New("conflicting user entries")
		}
		i = slackIndex
	}

	return i, nil
}

func SelectUser(_ context.Context, indexType int, id string) (User, error) {
	mu := getDataFileMutex(usersFile)
	mu.Lock()
	defer mu.Unlock()

	if err := initUsersDBIfNeeded(); err != nil {
		return User{}, err
	}

	var index map[string]int
	switch indexType {
	case IndexByEmail:
		index = usersDB.emailIndex
	case IndexByBitbucketID:
		index = usersDB.bitbucketIndex
	case IndexByGitHubID:
		index = usersDB.githubIndex
	case IndexBySlackID:
		index = usersDB.slackIndex
	case IndexByRealName:
		index = usersDB.nameIndex
	default:
		return User{}, errors.New("invalid index type")
	}

	i, found := index[id]
	if !found || i < 0 { // Negative index means non-unique (for names).
		return User{}, nil
	}

	entryCopy := usersDB.entries[i]
	return entryCopy, nil
}

// readUsersFile reads and indexes the users data file into an in-memory structure.
// This function expects the caller to hold the appropriate mutex.
func readUsersFile() (*Users, error) {
	path, err := dataPath(usersFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get data file path: %w", err)
	}

	db := &Users{entries: []User{}}
	f, err := os.Open(path) //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return db, nil
		}
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&db.entries); err != nil {
		return nil, fmt.Errorf("failed to read/decode data file: %w", err)
	}

	// Build indexes for fast lookups.
	db.emailIndex = map[string]int{}
	db.bitbucketIndex = map[string]int{}
	db.githubIndex = map[string]int{}
	db.slackIndex = map[string]int{}
	db.nameIndex = map[string]int{}

	for i, user := range db.entries {
		if user.Email != "" && user.Email != "bot" {
			db.entries[i].Email = strings.ToLower(db.entries[i].Email)
			db.emailIndex[db.entries[i].Email] = i
		}
		if user.BitbucketID != "" {
			db.bitbucketIndex[user.BitbucketID] = i
		}
		if user.GitHubID != "" {
			db.githubIndex[user.GitHubID] = i
		}
		if user.SlackID != "" {
			db.slackIndex[user.SlackID] = i
		}
		if user.RealName != "" {
			if _, found := db.nameIndex[user.RealName]; found {
				db.nameIndex[user.RealName] = -1 // Mark this name as non-unique.
				continue
			}
			db.nameIndex[user.RealName] = i
		}
	}

	return db, nil
}

// writeUsersFile expects the caller to hold the appropriate mutex.
func (u *Users) writeUsersFile() error {
	path, err := dataPath(usersFile)
	if err != nil {
		return fmt.Errorf("failed to get data file path: %w", err)
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		return fmt.Errorf("failed to open data file: %w", err)
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(u.entries)
}
