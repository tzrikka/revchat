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

func TestURLsIDsThreadDeletion(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache = map[string]string{} // Reset global state.

	// Initialization.
	if err := MapURLAndID("url1", "channel1"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}
	if err := MapURLAndID("url1/comment1", "channel1/msg1"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}
	if err := MapURLAndID("url1/comment2", "channel1/msg2"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}
	if err := MapURLAndID("url1/comment3", "channel1/msg2/thread1"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}

	if err := MapURLAndID("url2", "channel2"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}
	if err := MapURLAndID("url2/comment1", "channel2/msg1"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}
	if err := MapURLAndID("url2/comment2", "channel2/msg2"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}
	if err := MapURLAndID("url2/comment3", "channel2/msg2/thread1"); err != nil {
		t.Fatalf("MapURLAndID() error = %v", err)
	}

	// Remove "url1" and "channel1" mappings.
	if err := DeleteURLAndIDMapping("url1"); err != nil {
		t.Fatalf("DeleteURLAndIDMapping() error = %v", err)
	}

	// Check remaining mappings.
	tests := []struct {
		name  string
		param string
		want  string
	}{
		// Should be deleted.
		{"url1", "url1", ""},
		{"url1_comment1", "url1/comment1", ""},
		{"url1_comment2", "url1/comment2", ""},
		{"url1_comment3", "url1/comment3", ""},

		{"channel1", "channel1", ""},
		{"channel1_msg1", "channel1/msg1", ""},
		{"channel1_msg2", "channel1/msg2", ""},
		{"channel1_msg2_thread1", "channel1/msg2/thread1", ""},

		// Should remain.
		{"url2", "url2", "channel2"},
		{"url2_comment1", "url2/comment1", "channel2/msg1"},
		{"url2_comment2", "url2/comment2", "channel2/msg2"},
		{"url2_comment3", "url2/comment3", "channel2/msg2/thread1"},

		{"channel2", "channel2", "url2"},
		{"channel2_msg1", "channel2/msg1", "url2/comment1"},
		{"channel2_msg2", "channel2/msg2", "url2/comment2"},
		{"channel2_msg2_thread1", "channel2/msg2/thread1", "url2/comment3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SwitchURLAndID(tt.param)
			if err != nil {
				t.Errorf("SwitchURLAndID() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("SwitchURLAndID() = %q, want %q", got, tt.want)
			}
		})
	}
}
