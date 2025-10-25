package data

import (
	"fmt"
	"os"
	"sync"

	"github.com/BurntSushi/toml"
)

const (
	remindersFile = "reminders.toml"
)

func SetReminder(userID, kitchenTime, tz string) error {
	path := dataPath(remindersFile)

	m, err := readTOML(path)
	if err != nil {
		return err
	}

	m[userID] = fmt.Sprintf("%s %s", kitchenTime, tz)
	return writeTOML(path, m)
}

func DeleteReminder(userID string) error {
	path := dataPath(remindersFile)

	m, err := readTOML(path)
	if err != nil {
		return err
	}

	delete(m, userID)
	return writeTOML(path, m)
}

func ListReminders() (map[string]string, error) {
	return readTOML(dataPath(remindersFile))
}

var mu sync.RWMutex

func readTOML(path string) (map[string]string, error) {
	mu.RLock()
	defer mu.RUnlock()

	m := map[string]string{}
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeTOML(path string, m map[string]string) error {
	mu.Lock()
	defer mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 -- specified by admin by design
	if err != nil {
		return err
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(m); err != nil {
		return err
	}
	return nil
}
