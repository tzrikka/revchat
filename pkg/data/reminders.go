package data

import (
	"fmt"
	"sync"
)

const (
	remindersFile = "reminders.json"
)

func SetReminder(userID, kitchenTime, tz string) error {
	m, err := readRemindersFile()
	if err != nil {
		return err
	}

	m[userID] = fmt.Sprintf("%s %s", kitchenTime, tz)
	return writeRemindersFile(m)
}

func DeleteReminder(userID string) error {
	m, err := readRemindersFile()
	if err != nil {
		return err
	}

	delete(m, userID)
	return writeRemindersFile(m)
}

func ListReminders() (map[string]string, error) {
	return readRemindersFile()
}

var remindersMutex sync.RWMutex

func readRemindersFile() (map[string]string, error) {
	remindersMutex.RLock()
	defer remindersMutex.RUnlock()

	return readJSON(remindersFile)
}

func writeRemindersFile(m map[string]string) error {
	remindersMutex.Lock()
	defer remindersMutex.Unlock()

	return writeJSON(remindersFile, m)
}
