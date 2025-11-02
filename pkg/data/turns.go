package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

// PRTurn represents the turn-taking state for a specific pull request.
type PRTurn struct {
	Author    string          `json:"author"`    // Email address of the PR author.
	Reviewers map[string]bool `json:"reviewers"` // Email address -> is it their turn?
	Set       bool            `json:"set"`       // Whether the turn-taking state has been set explicitly.
}

// InitTurn initializes the turn-taking state of a new PR.
// Users are specified using their email addresses.
func InitTurn(url, author string, reviewers []string) error {
	t := &PRTurn{
		Author:    author,
		Reviewers: make(map[string]bool, len(reviewers)),
	}

	for _, reviewer := range reviewers {
		t.Reviewers[reviewer] = true
	}

	return saveTurn(url, t)
}

// DeleteTurn removes the turn-taking state file of a specific PR.
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

	t, err := loadTurn(url)
	if err != nil {
		return err
	}

	if _, found := t.Reviewers[email]; found || email == t.Author {
		return nil
	}
	t.Reviewers[email] = true

	return saveTurn(url, t)
}

// GetCurrentTurn returns the email addresses of all the users whose turn it is to
// pay attention to a specific PR. If the PR has no assigned reviewers, this function
// returns the PR author (as a reminder for them to assign reviewers). If any assigned
// reviewer has their turn flag set to false, we add the author to the list as well,
// unless the attention list was set explicitly using [SetTurn].
func GetCurrentTurn(url string) ([]string, error) {
	t, err := loadTurn(url)
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

	t, err := loadTurn(url)
	if err != nil {
		return err
	}

	if _, found := t.Reviewers[email]; !found {
		return nil
	}
	delete(t.Reviewers, email)

	return saveTurn(url, t)
}

// SetTurn overwrites the turn-taking state of a specific PR to an explicit set of users.
// Missing users are added, which means the caller needs to ensure they *are* valid reviewers.
// Existing unmentioned users are not removed, but are marked as not their turn. If the
// input contains the PR author, they *are* added (temporarily, until [SwitchTurn]
// is called for them). It also ignores empty or "bot" email addresses.
func SetTurn(url string, emails []string) error {
	t, err := loadTurn(url)
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
	return saveTurn(url, t)
}

// SwitchTurn switches the turn of a specific user in a specific PR.
// If the user is the PR author, it adds all reviewers to the attention list.
// If the user is a reviewer, it adds the author to the attention list.
// If the user is not found, this function does nothing.
func SwitchTurn(url, email string) error {
	if email == "" || email == "bot" {
		return nil
	}

	t, err := loadTurn(url)
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
	return saveTurn(url, t)
}

// turnPath returns the absolute path to the JSON file representing the turn-taking state of a PR.
// This function is different from [dataPath] because it supports subdirectories.
// It creates any necessary parent directories, but not the file itself.
func turnPath(url string) string {
	prefix, _ := xdg.CreateDir(xdg.DataHome, config.DirName)
	suffix, _ := strings.CutPrefix(url, "https://")
	filePath := filepath.Clean(filepath.Join(prefix, suffix))

	_ = os.MkdirAll(filepath.Dir(filePath), xdg.NewDirectoryPermissions)

	return filePath + "_turn.json"
}

func saveTurn(url string, t *PRTurn) error {
	f, err := os.OpenFile(turnPath(url), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- verified URL
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(t)
}

func loadTurn(url string) (*PRTurn, error) {
	f, err := os.Open(turnPath(url)) //gosec:disable G304 -- URL received from verified 3rd-party
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
