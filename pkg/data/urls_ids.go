package data

import (
	"bytes"
	"encoding/json"
	"os"
)

const (
	urlsFile = "urls_ids.json"
)

// MapURLAndID saves a 2-way mapping between a PR URL
// and its dedicated chat channel or thread IDs.
func MapURLAndID(url, id string) error {
	path := dataPath(urlsFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	m[url] = id
	m[id] = url
	return writeJSON(path, m)
}

// SwitchURLAndID converts a PR URL to its mapped
// chat channel or thread IDs, and vice versa.
func SwitchURLAndID(key string) (string, error) {
	m, err := readJSON(dataPath(urlsFile))
	if err != nil {
		return "", err
	}

	return m[key], nil
}

// DeleteURLAndIDMapping deletes the 2-way mapping between PR URLs and chat channel and thread
// IDs when they become obsolete (i.e. when the PR is merged, closed, or marked as a draft).
func DeleteURLAndIDMapping(key string) error {
	path := dataPath(urlsFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	delete(m, m[key])
	delete(m, key)
	return writeJSON(path, m)
}

func readJSON(path string) (map[string]string, error) {
	f, err := os.ReadFile(path) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return nil, err
	}

	// Special case: empty files can't be parsed as JSON,
	// but this initial state is valid.
	m := map[string]string{}
	if len(f) == 0 {
		return m, nil
	}

	if err := json.NewDecoder(bytes.NewReader(f)).Decode(&m); err != nil {
		return nil, err
	}

	return m, nil
}

func writeJSON(path string, m map[string]string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(m)
}
