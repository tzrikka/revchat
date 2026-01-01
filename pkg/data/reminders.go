package data

import (
	"context"
	"fmt"
	"sync"

	"go.temporal.io/sdk/workflow"
)

const (
	remindersFile = "reminders.json"
)

var remindersMutex sync.RWMutex

func SetReminder(ctx workflow.Context, userID, kitchenTime, tz string) error {
	m, err := ListReminders(ctx)
	if err != nil {
		return err
	}

	m[userID] = fmt.Sprintf("%s %s", kitchenTime, tz)

	remindersMutex.Lock()
	defer remindersMutex.Unlock()

	if ctx == nil { // For unit testing.
		return writeJSONActivity(context.Background(), remindersFile, m)
	}
	return executeLocalActivity(ctx, writeJSONActivity, nil, remindersFile, m)
}

func DeleteReminder(ctx workflow.Context, userID string) error {
	m, err := ListReminders(ctx)
	if err != nil {
		return err
	}

	delete(m, userID)

	remindersMutex.Lock()
	defer remindersMutex.Unlock()

	if ctx == nil { // For unit testing.
		return writeJSONActivity(context.Background(), remindersFile, m)
	}
	return executeLocalActivity(ctx, writeJSONActivity, nil, remindersFile, m)
}

func ListReminders(ctx workflow.Context) (map[string]string, error) {
	remindersMutex.RLock()
	defer remindersMutex.RUnlock()

	if ctx == nil { // For unit testing.
		return readJSONActivity(context.Background(), remindersFile)
	}

	file := map[string]string{}
	if err := executeLocalActivity(ctx, readJSONActivity, &file, remindersFile); err != nil {
		return nil, err
	}
	return file, nil
}
