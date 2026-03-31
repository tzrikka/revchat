package internal

import (
	"context"
	"fmt"
)

const (
	remindersFile = "reminders.json"
)

func SetReminder(_ context.Context, userID, kitchenTime, tz string) error {
	mu := getDataFileMutex(remindersFile)
	mu.Lock()
	defer mu.Unlock()

	m, err := readGenericJSONFile(remindersFile)
	if err != nil {
		return err
	}

	m[userID] = fmt.Sprintf("%s %s", kitchenTime, tz)
	return writeGenericJSONFile(remindersFile, m)
}

func DeleteReminder(_ context.Context, userID string) error {
	mu := getDataFileMutex(remindersFile)
	mu.Lock()
	defer mu.Unlock()

	m, err := readGenericJSONFile(remindersFile)
	if err != nil {
		return err
	}

	delete(m, userID)
	return writeGenericJSONFile(remindersFile, m)
}

func ListReminders(_ context.Context) (map[string]string, error) {
	mu := getDataFileMutex(remindersFile)
	mu.Lock()
	defer mu.Unlock()

	return readGenericJSONFile(remindersFile)
}
