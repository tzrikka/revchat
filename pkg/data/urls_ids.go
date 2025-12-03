package data

import (
	"strings"
	"sync"
)

const (
	urlsIDsFile = "urls_ids.json"
)

// MapURLAndID saves a 2-way mapping between a PR URL
// and its dedicated chat channel or thread IDs.
func MapURLAndID(url, id string) error {
	m, err := readURLsIDsFile()
	if err != nil {
		return err
	}

	m[url] = id
	m[id] = url
	return writeURLsIDsFile(m)
}

// SwitchURLAndID converts a PR URL to its mapped
// chat channel or thread IDs, and vice versa.
func SwitchURLAndID(key string) (string, error) {
	m, err := readURLsIDsFile()
	if err != nil {
		return "", err
	}

	return m[key], nil
}

// DeleteURLAndIDMapping deletes the 2-way mapping between PR URLs and chat channel and thread
// IDs when they become obsolete (i.e. when the PR is merged, closed, or marked as a draft).
func DeleteURLAndIDMapping(key string) error {
	m, err := readURLsIDsFile()
	if err != nil {
		return err
	}

	delete(m, m[key])
	delete(m, key)

	prefix := key + "/"
	var moreKeysToDelete []string
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			moreKeysToDelete = append(moreKeysToDelete, k, m[k])
		}
	}
	for _, k := range moreKeysToDelete {
		delete(m, k)
	}

	return writeURLsIDsFile(m)
}

var urlsIDsMutex sync.RWMutex

func readURLsIDsFile() (map[string]string, error) {
	urlsIDsMutex.RLock()
	defer urlsIDsMutex.RUnlock()

	return readJSON(urlsIDsFile)
}

func writeURLsIDsFile(m map[string]string) error {
	urlsIDsMutex.Lock()
	defer urlsIDsMutex.Unlock()

	return writeJSON(urlsIDsFile, m)
}
