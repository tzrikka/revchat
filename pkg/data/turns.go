package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

// PRTurn represents the attention state for a specific pull request.
//
// The [Reviewers] map tracks which reviewers are assigned to the PR
// and whether it's currently their turn to pay attention to it.
//
// Initial state:
//
//   - If the map is empty, the PR author is considered responsible for
//     the PR's progress: they need to assign reviewers or merge the PR.
//   - When any number of reviewers are assigned, at the same time or
//     separately, the initial state of their turn flag is set to true.
//     Each one of them needs to pay attention to the PR.
//
// State transitions for reviewers:
//
//   - When a reviewer approves the PR, or is unassigned from it, they
//     are removed from the map. They will no longer need to pay
//     attention to this PR, unless they are added again later.
//   - When a reviewer creates a new comment, reply, or code suggestion,
//     their turn flag is set to false. It is no longer their turn to
//     pay attention to the PR. This does not affect other reviewers.
//   - If a reviewer creates multiple-but-separate comments or code
//     suggestions, their turn flag remains false.
//
// State transitions for the author:
//
//   - The author's state is USUALLY not tracked explicitly. They need
//     to pay attention to the PR whenever at least one reviewer has
//     their turn flag set to false.
//
//   - When the author addresses review comments (i.e. creates a new
//     comment or reply), the turn flags of all the reviewers are reset
//     to true. It is their turn still/again to pay attention to the PR.
//
//   - Pushing commits does not have the same effect for now, as it may
//     be work in progress. Only discussions trigger state changes.
//
// Manual state changes:
//
//   - Any participant (author or reviewers) may indicate that it is
//     or isn't their turn to pay attention to the PR, by using a Slack
//     slash command instead of interacting within the PR discussion.
//   - Any participant may also set the entire attention state explicitly,
//     specifying exactly which reviewers need to pay attention to the PR.
//     Unmentioned reviewers are not removed from the map, but their turn
//     flags are set to false. If the author is mentioned, they are added
//     temporarily until a regular state transition affects them.
type PRTurn struct {
	Author    string          `json:"author"`    // Email address of the PR author.
	Reviewers map[string]bool `json:"reviewers"` // Email address -> is it their turn?
	Set       bool            `json:"set"`       // Whether the attention state has been set explicitly.
}

// InitTurn initializes the attention state of a new PR.
// Users are specified using their email addresses.
func InitTurn(url, author string, reviewers []string) error {
	t := &PRTurn{
		Author:    author,
		Reviewers: make(map[string]bool, len(reviewers)),
	}

	for _, reviewer := range reviewers {
		t.Reviewers[reviewer] = true
	}

	return writeTurnFile(url, t)
}

// DeleteTurn removes the attention state file of a specific PR.
// This is used when the PR is closed or marked as a draft.
func DeleteTurn(url string) error {
	return os.Remove(turnPath(url))
}

// AddReviewerToPR adds a new reviewer to the attention list of a specific PR.
// This function is idempotent: if a reviewer already exists, or is the PR author,
// it does nothing. It also ignores empty or "bot" email addresses.
func AddReviewerToPR(url, email string) error {
	if email == "" || email == "bot" {
		return nil
	}

	t, err := readTurnFile(url)
	if err != nil {
		return err
	}

	if _, found := t.Reviewers[email]; found || email == t.Author {
		return nil
	}
	t.Reviewers[email] = true

	return writeTurnFile(url, t)
}

// GetCurrentTurn returns the email addresses of all the users whose turn it is to
// pay attention to a specific PR. If the PR has no assigned reviewers, this function
// returns the PR author (as a reminder for them to assign reviewers). If any assigned
// reviewer has their turn flag set to false, we add the author to the list as well,
// unless the attention list was set explicitly using [SetTurn].
func GetCurrentTurn(url string) ([]string, error) {
	t, err := readTurnFile(url)
	if err != nil {
		return nil, err
	}

	if len(t.Reviewers) == 0 {
		return []string{t.Author}, nil
	}

	emails := make([]string, 0, len(t.Reviewers)+1)
	includeAuthor := false
	for email, isTurn := range t.Reviewers {
		if isTurn {
			emails = append(emails, email)
		} else {
			includeAuthor = true
		}
	}
	if !t.Set && includeAuthor {
		emails = append(emails, t.Author)
	}

	slices.Sort(emails)
	return emails, nil
}

// RemoveFromTurn completely removes a reviewer from the attention list of a specific PR.
// This is used when that reviewer approves the PR, or is unassigned from the PR.
// This function is idempotent: if the reviewer does not exist, it does nothing.
// It also ignores empty or "bot" email addresses.
func RemoveFromTurn(url, email string) error {
	if email == "" || email == "bot" {
		return nil
	}

	t, err := readTurnFile(url)
	if err != nil {
		return err
	}

	if _, found := t.Reviewers[email]; !found {
		return nil
	}
	delete(t.Reviewers, email)

	return writeTurnFile(url, t)
}

// SetTurn overwrites the attention state of a specific PR to an explicit set of users.
// Missing users are added, which means the caller needs to ensure they *are* valid reviewers.
// Existing unmentioned users are not removed, but are marked as not their turn. If the
// input contains the PR author, they *are* added (temporarily, until [SwitchTurn]
// is called for them). It also ignores empty or "bot" email addresses.
func SetTurn(url string, emails []string) error {
	t, err := readTurnFile(url)
	if err != nil {
		return err
	}

	for email := range t.Reviewers {
		t.Reviewers[email] = false
	}

	for _, email := range emails {
		if email == "" || email == "bot" {
			continue
		}
		t.Reviewers[email] = true
	}

	t.Set = true
	return writeTurnFile(url, t)
}

// SwitchTurn switches the turn of a specific user in a specific PR.
// If the user is the PR author, it adds all reviewers to the attention list.
// If the user is a reviewer, it adds the author to the attention list.
// If the user is not found, this function does nothing.
func SwitchTurn(url, email string) error {
	if email == "" || email == "bot" {
		return nil
	}

	t, err := readTurnFile(url)
	if err != nil {
		return err
	}

	if email == t.Author {
		delete(t.Reviewers, email) // In case the author was added via [SetTurn].
		for reviewer := range t.Reviewers {
			t.Reviewers[reviewer] = true
		}
	} else {
		if _, found := t.Reviewers[email]; found {
			t.Reviewers[email] = false
		}
	}

	t.Set = false
	return writeTurnFile(url, t)
}

var turnMutexes = map[string]*sync.RWMutex{}

// turnPath returns the absolute path to the JSON file representing the attention state of a PR.
// This function is different from [xdg.CreateFile] because it supports subdirectories.
// It creates any necessary parent directories, but not the file itself.
func turnPath(url string) string {
	prefix, _ := xdg.CreateDir(xdg.DataHome, config.DirName)
	suffix, _ := strings.CutPrefix(url, "https://")
	filePath := filepath.Clean(filepath.Join(prefix, suffix))

	_ = os.MkdirAll(filepath.Dir(filePath), xdg.NewDirectoryPermissions)

	return filePath + "_turn.json"
}

func readTurnFile(url string) (*PRTurn, error) {
	if _, ok := turnMutexes[url]; !ok {
		turnMutexes[url] = &sync.RWMutex{}
	}
	turnMutexes[url].RLock()
	defer turnMutexes[url].RUnlock()

	path, err := cachedDataPath(url)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path) //gosec:disable G304 -- URL received from signature-verified 3rd-party
	if err != nil {
		return nil, err
	}
	defer f.Close()

	t := new(PRTurn)
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return nil, err
	}

	return t, nil
}

func writeTurnFile(url string, t *PRTurn) error {
	if _, ok := turnMutexes[url]; !ok {
		turnMutexes[url] = &sync.RWMutex{}
	}
	turnMutexes[url].Lock()
	defer turnMutexes[url].Unlock()

	path, err := cachedDataPath(url)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 -- URL received from signature-verified 3rd-party
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(t)
}
