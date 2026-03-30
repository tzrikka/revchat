package data_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/data"
)

func TestPRSnapshot(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	prURL := "https://bitbucket.org/workspace/repo/pull-requests/12345"

	// Initial state.
	got, err := data.LoadPRSnapshot(nil, prURL)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadPRSnapshot() = %#v, want %v", got, nil)
	}

	// Initial snapshot.
	pr1 := map[string]any{"title": "pr1"}
	data.StorePRSnapshot(nil, prURL, pr1)

	got, err = data.LoadPRSnapshot(nil, prURL)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(got, pr1) {
		t.Fatalf("LoadPRSnapshot() = %#v, want %#v", got, pr1)
	}

	// Update snapshot.
	pr2 := map[string]any{"title": "pr2"}
	data.StorePRSnapshot(nil, prURL, pr2)

	got, err = data.LoadPRSnapshot(nil, prURL)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(got, pr2) {
		t.Fatalf("LoadPRSnapshot() = %#v, want %#v", got, pr2)
	}

	// Delete snapshot.
	data.DeletePRSnapshot(nil, prURL)

	got, err = data.LoadPRSnapshot(nil, prURL)
	if err != nil {
		t.Fatalf("LoadPRSnapshot() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadPRSnapshot() = %#v, want %v", got, nil)
	}
}
