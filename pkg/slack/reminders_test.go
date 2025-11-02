package slack

import (
	"testing"
	"time"
)

func TestReminderTimes(t *testing.T) {
	startTime := time.Date(2025, 12, 20, 13, 0, 0, 0, time.UTC)

	_, gotTime, gotErr := reminderTimes(nil, startTime, "userID", "8:00AM America/New_York")
	if gotErr != nil {
		t.Fatalf("reminderTimes() error: %v", gotErr)
	}

	hours, mins, secs := gotTime.Clock()
	if hours != 8 || mins != 0 || secs != 0 {
		t.Errorf("reminderTimes() = %v, want 08:00:00", gotTime)
	}
}
