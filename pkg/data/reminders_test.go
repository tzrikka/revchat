package data

import (
	"fmt"
	"testing"
)

func TestReminders(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache = map[string]string{} // Reset global state.

	uid1 := "U123456"
	kt1 := "9:00AM"
	tz1 := "America/Los_Angeles"
	reminder1 := fmt.Sprintf("%s %s", kt1, tz1)

	kt2 := "5:00PM"
	tz2 := "America/Los_Angeles"
	reminder2 := fmt.Sprintf("%s %s", kt2, tz2)

	// Before mapping.
	got, err := ListReminders(nil)
	if err != nil {
		t.Fatalf("ListReminders() error = %v", err)
	}
	if len(got) > 0 {
		t.Fatalf("ListReminders() = %v, want empty/nil", got)
	}

	// Set a reminder.
	if err := SetReminder(nil, uid1, kt1, tz1); err != nil {
		t.Fatalf("SetReminder() error = %v", err)
	}

	// After mapping.
	got, err = ListReminders(nil)
	if err != nil {
		t.Fatalf("ListReminders() error = %v", err)
	}
	want := map[string]string{uid1: reminder1}
	if len(got) != 1 {
		t.Fatalf("ListReminders() = %v, want %v", got, want)
	}
	if got[uid1] != want[uid1] {
		t.Fatalf("ListReminders()[key] = %v, want %v", got[uid1], want[uid1])
	}

	// Modify mapping.
	if err := SetReminder(nil, uid1, kt2, tz2); err != nil {
		t.Fatalf("SetReminder() error = %v", err)
	}
	got, err = ListReminders(nil)
	if err != nil {
		t.Fatalf("ListReminders() error = %v", err)
	}
	want = map[string]string{uid1: reminder2}
	if len(got) != 1 {
		t.Fatalf("ListReminders() = %v, want %v", got, want)
	}
	if got[uid1] != want[uid1] {
		t.Fatalf("ListReminders()[key] = %v, want %v", got[uid1], want[uid1])
	}

	// Remove mapping.
	if err := DeleteReminder(nil, uid1); err != nil {
		t.Fatalf("DeleteReminder() error = %v", err)
	}

	// After removal.
	got, err = ListReminders(nil)
	if err != nil {
		t.Fatalf("ListReminders() error = %v", err)
	}
	if len(got) > 0 {
		t.Fatalf("ListReminders() = %v, want empty/nil", got)
	}
}
