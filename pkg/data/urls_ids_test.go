package data

import (
	"testing"
)

func TestURLsIDs(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache = map[string]string{} // Reset global state.

	url := "url"
	channel := "channel"

	// Before mapping.
	got, err := SwitchURLAndID(url)
	if err != nil {
		t.Fatalf("SwitchURLAndID() error = %v", err)
	}
	want := ""
	if got != want {
		t.Fatalf("SwitchURLAndID() = %q, want %q", got, want)
	}

	// Map PR URL to Slack channel.
	if err := MapURLAndID(url, channel); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}

	// After mapping.
	got, err = SwitchURLAndID(url)
	if err != nil {
		t.Fatalf("SwitchURLAndID() error = %v", err)
	}
	want = channel
	if got != want {
		t.Fatalf("SwitchURLAndID() = %q, want %q", got, want)
	}

	// Remove mapping.
	if err := DeleteURLAndIDMapping("url"); err != nil {
		t.Fatalf("DeleteURLAndIDMapping() error = %v", err)
	}

	// After removal.
	got, err = SwitchURLAndID(url)
	if err != nil {
		t.Fatalf("SwitchURLAndID() error = %v", err)
	}
	want = ""
	if got != want {
		t.Fatalf("SwitchURLAndID() = %q, want %q", got, want)
	}
}
