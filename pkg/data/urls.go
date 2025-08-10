package data

import (
	"bytes"
	"encoding/json"
	"os"
)

const (
	urlsFile = "urls.json"
)

func MapURLToChannel(url, channel string) error {
	path := dataPath(urlsFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	m[url] = channel
	return writeJSON(path, m)
}

func ConvertURLToChannel(url string) (string, error) {
	m, err := readJSON(dataPath(urlsFile))
	if err != nil {
		return "", err
	}

	return m[url], nil
}

func RemoveURLToChannelMapping(url string) error {
	path := dataPath(urlsFile)

	m, err := readJSON(path)
	if err != nil {
		return err
	}

	delete(m, url)
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
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- user-specified by design
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(m)
}
