// Package data provides functions to manage persistent data mappings.
// It is entirely inefficient and should be replaced with a proper solution.
package data

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	urlsFileName = "urls.json"
)

// dataPath returns the absolute path to the given data file.
// It also creates an empty file if it doesn't already exist.
func dataPath(filename string) string {
	path, _ := xdg.CreateFile(xdg.DataHome, config.DirName, filename)
	return path
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

func writeSON(path string, m map[string]string) error {
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(m)
}

func MapURLToChannel(url, channel string) error {
	path := dataPath(urlsFileName)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	m[url] = channel
	return writeSON(path, m)
}

func ConvertURLToChannel(url string) (string, error) {
	m, err := readJSON(dataPath(urlsFileName))
	if err != nil {
		return "", err
	}
	return m[url], nil
}

func RemoveURLToChannelMapping(url string) error {
	path := dataPath(urlsFileName)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	delete(m, url)
	return writeSON(path, m)
}
