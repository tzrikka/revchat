package data

import (
	"reflect"
	"testing"
)

func TestPRSnapshot(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache.Clear()

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initial state.
	got, err := LoadPRSnapshot(nil, url)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadPRSnapshot() = %#v, want %#v", got, nil)
	}

	// Initial snapshot.
	pr1 := map[string]any{"title": "pr1"}

	StorePRSnapshot(nil, url, pr1)

	got, err = LoadPRSnapshot(nil, url)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(got, pr1) {
		t.Fatalf("LoadPRSnapshot() = %v, want %v", got, pr1)
	}

	// Update snapshot.
	pr2 := map[string]any{"title": "pr2"}

	StorePRSnapshot(nil, url, pr2)

	got, err = LoadPRSnapshot(nil, url)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(got, pr2) {
		t.Fatalf("LoadPRSnapshot() = %v, want %v", got, pr2)
	}

	// Delete snapshot.
	DeletePRSnapshot(nil, url)

	got, err = LoadPRSnapshot(nil, url)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadPRSnapshot() = %v, want %v", got, nil)
	}
}
