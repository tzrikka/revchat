package data

import (
	"testing"
)

func TestURLs(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "url"
	channel := "channel"

	// Before mapping.
	got, err := ConvertURLToChannel(url)
	if err != nil {
		t.Fatalf("ConvertURLToChannel() error = %v", err)
	}
	want := ""
	if got != want {
		t.Fatalf("ConvertURLToChannel() = %q, want %q", got, want)
	}

	// Map PR URL to Slack channel.
	if err := MapURLToChannel(url, channel); err != nil {
		t.Fatalf("MapURLToChannel() error = %v", err)
	}

	// After mapping.
	got, err = ConvertURLToChannel(url)
	if err != nil {
		t.Fatalf("ConvertURLToChannel() error = %v", err)
	}
	want = channel
	if got != want {
		t.Fatalf("ConvertURLToChannel() = %q, want %q", got, want)
	}

	// Remove mapping.
	if err := RemoveURLToChannelMapping("url"); err != nil {
		t.Fatalf("RemoveURLToChannelMapping() error = %v", err)
	}

	// After removal.
	got, err = ConvertURLToChannel(url)
	if err != nil {
		t.Fatalf("ConvertURLToChannel() error = %v", err)
	}
	want = ""
	if got != want {
		t.Fatalf("ConvertURLToChannel() = %q, want %q", got, want)
	}
}
