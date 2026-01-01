package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

const (
	usersFile = "users.json"

	indexByEmail       = 1
	indexByBitbucketID = 2
	indexByGitHubID    = 3
	indexBySlackID     = 4
	indexByRealName    = 5
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

	// Slack user IDs, controlled by the un/follow slash commands, used when creating channels.
	Followers []string `json:"followers,omitempty"`

	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
	Deleted string `json:"deleted,omitempty"`
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

var (
	usersDB    *Users
	usersMutex sync.Mutex
)

func UpsertUser(ctx workflow.Context, email, realName, bitbucketID, githubID, slackID, thrippyLink string) error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	if ctx == nil { // For unit tests.
		return upsertUserActivity(context.Background(), email, realName, bitbucketID, githubID, slackID, thrippyLink)
	}

	err := executeLocalActivity(ctx, upsertUserActivity, nil, email, realName, bitbucketID, githubID, slackID, thrippyLink)
	if err != nil {
		logger.From(ctx).Error("failed to upsert user data", slog.Any("error", err),
			slog.String("email", email), slog.String("real_name", realName),
			slog.String("bitbucket_id", bitbucketID), slog.String("github_id", githubID),
			slog.String("slack_id", slackID), slog.String("thrippy_link", thrippyLink))
		return fmt.Errorf("failed to upsert user data: %w", err)
	}
	return nil
}

func FollowUser(ctx workflow.Context, followerSlackID, followedSlackID string) bool {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	if err := executeLocalActivity(ctx, followUserActivity, nil, followerSlackID, followedSlackID); err != nil {
		logger.From(ctx).Error("failed to follow user", slog.Any("error", err),
			slog.String("follower_id", followerSlackID), slog.String("followed_id", followedSlackID))
		return false
	}
	return true
}

func UnfollowUser(ctx workflow.Context, followerSlackID, followedSlackID string) bool {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	if err := executeLocalActivity(ctx, unfollowUserActivity, nil, followerSlackID, followedSlackID); err != nil {
		logger.From(ctx).Error("failed to unfollow user", slog.Any("error", err),
			slog.String("follower_id", followerSlackID), slog.String("followed_id", followedSlackID))
		return false
	}
	return true
}

func SelectUserByBitbucketID(ctx workflow.Context, accountID string) User {
	if accountID == "" {
		return User{}
	}

	usersMutex.Lock()
	defer usersMutex.Unlock()

	if ctx == nil { // For unit tests.
		user, _ := selectUserActivity(nil, indexByBitbucketID, accountID)
		return user
	}

	user := new(User)
	if err := executeLocalActivity(ctx, selectUserActivity, user, indexByBitbucketID, accountID); err != nil {
		logger.From(ctx).Warn("unexpected but not critical: failed to load user data by Bitbucket ID",
			slog.Any("error", err), slog.String("account_id", accountID))
		return User{}
	}
	return *user
}

func SelectUserByEmail(ctx workflow.Context, email string) User {
	if email == "" {
		return User{}
	}

	usersMutex.Lock()
	defer usersMutex.Unlock()

	if ctx == nil { // For unit tests.
		user, _ := selectUserActivity(nil, indexByEmail, strings.ToLower(email))
		return user
	}

	user := new(User)
	if err := executeLocalActivity(ctx, selectUserActivity, user, indexByEmail, strings.ToLower(email)); err != nil {
		logger.From(ctx).Error("failed to load user data by email", slog.Any("error", err), slog.String("email", email))
		return User{}
	}
	return *user
}

func SelectUserByGitHubID(ctx workflow.Context, login string) User {
	if login == "" {
		return User{}
	}

	usersMutex.Lock()
	defer usersMutex.Unlock()

	if ctx == nil { // For unit tests.
		user, _ := selectUserActivity(nil, indexByGitHubID, login)
		return user
	}

	user := new(User)
	if err := executeLocalActivity(ctx, selectUserActivity, user, indexByGitHubID, strings.ToLower(login)); err != nil {
		logger.From(ctx).Error("failed to load user data by GitHub ID", slog.Any("error", err), slog.String("login", login))
		return User{}
	}
	return *user
}

func SelectUserBySlackID(ctx workflow.Context, userID string) (User, bool, error) {
	if userID == "" {
		return User{}, false, nil
	}

	usersMutex.Lock()
	defer usersMutex.Unlock()

	if ctx == nil { // For unit tests.
		user, err := selectUserActivity(nil, indexBySlackID, userID)
		return user, user.ThrippyLink != "", err
	}

	user := new(User)
	if err := executeLocalActivity(ctx, selectUserActivity, user, indexBySlackID, userID); err != nil {
		logger.From(ctx).Error("failed to load user data by Slack ID", slog.Any("error", err), slog.String("user_id", userID))
		return User{}, false, err
	}

	return *user, user.ThrippyLink != "", nil
}

func SelectUserByRealName(ctx workflow.Context, realName string) User {
	if realName == "" {
		return User{}
	}

	usersMutex.Lock()
	defer usersMutex.Unlock()

	if ctx == nil { // For unit tests.
		user, _ := selectUserActivity(nil, indexByRealName, realName)
		return user
	}

	user := new(User)
	if err := executeLocalActivity(ctx, selectUserActivity, user, indexByRealName, realName); err != nil {
		logger.From(ctx).Error("failed to load user data by real name",
			slog.Any("error", err), slog.String("real_name", realName))
		return User{}
	}
	return *user
}

func upsertUserActivity(_ context.Context, email, realName, bitbucketID, githubID, slackID, thrippyLink string) error {
	if usersDB == nil {
		var err error
		usersDB, err = readUsersFile()
		if err != nil {
			return err
		}
	}

	i, err := usersDB.findUserIndex(email, realName, bitbucketID, githubID, slackID)
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
		if thrippyLink == "X" {
			thrippyLink = ""
		}
		usersDB.entries[i].ThrippyLink = thrippyLink
	}

	return usersDB.writeUsersFile()
}

func followUserActivity(_ context.Context, followerSlackID, followedSlackID string) error {
	if usersDB == nil {
		var err error
		usersDB, err = readUsersFile()
		if err != nil {
			return err
		}
	}

	i, err := usersDB.findUserIndex("", "", "", "", followedSlackID)
	if err != nil || i < 0 {
		return err
	}

	if !slices.Contains(usersDB.entries[i].Followers, followerSlackID) {
		usersDB.entries[i].Updated = time.Now().UTC().Format(time.RFC3339)
		usersDB.entries[i].Followers = append(usersDB.entries[i].Followers, followerSlackID)
		slices.Sort(usersDB.entries[i].Followers)
	}

	return usersDB.writeUsersFile()
}

func unfollowUserActivity(_ context.Context, followerSlackID, followedSlackID string) error {
	if usersDB == nil {
		var err error
		usersDB, err = readUsersFile()
		if err != nil {
			return err
		}
	}

	i, err := usersDB.findUserIndex("", "", "", "", followedSlackID)
	if err != nil || i < 0 {
		return err
	}

	j := slices.Index(usersDB.entries[i].Followers, followerSlackID)
	if j < 0 {
		return nil
	}

	usersDB.entries[i].Followers = slices.Delete(usersDB.entries[i].Followers, j, j+1)
	usersDB.entries[i].Updated = time.Now().UTC().Format(time.RFC3339)

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

func selectUserActivity(ctx workflow.Context, indexType int, id string) (User, error) {
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
	case indexByRealName:
		index = usersDB.nameIndex
	default:
		return User{}, errors.New("invalid index type")
	}

	i, found := index[id]
	if !found || i < 0 { // Negative index means non-unique (for names).
		return User{}, nil
	}

	entryCopy := usersDB.entries[i]
	entryCopy.Email = strings.ToLower(entryCopy.Email)
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
		f, err := os.Open(path) //gosec:disable G304 // Specified by admin by design.
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
	u.nameIndex = map[string]int{}

	for i, user := range u.entries {
		if user.Email != "" && user.Email != "bot" {
			user.Email = strings.ToLower(user.Email)
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
		if user.RealName != "" {
			if _, found := u.nameIndex[user.RealName]; found {
				u.nameIndex[user.RealName] = -1 // Mark this name as non-unique.
				continue
			}
			u.nameIndex[user.RealName] = i
		}
	}

	return u, nil
}

func (u *Users) writeUsersFile() error {
	path, err := cachedDataPath(usersFile, "")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(u.entries)
}
