package data

import (
	"reflect"
	"testing"
)

func TestBitbucket(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache.Clear()

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initial state.
	got, err := LoadBitbucketPR(nil, url)
	if err != nil {
		t.Fatalf("LoadBitbucketPR() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadBitbucketPR() = %v, want %v", got, nil)
	}

	// Initial snapshot.
	pr1 := map[string]any{"title": "pr1"}

	StoreBitbucketPR(nil, url, pr1)

	got, err = LoadBitbucketPR(nil, url)
	if err != nil {
		t.Fatalf("LoadBitbucketPR() error = %v", err)
	}
	if !reflect.DeepEqual(got, pr1) {
		t.Fatalf("LoadBitbucketPR() = %v, want %v", got, pr1)
	}

	// Update snapshot.
	pr2 := map[string]any{"title": "pr2"}

	StoreBitbucketPR(nil, url, pr2)

	got, err = LoadBitbucketPR(nil, url)
	if err != nil {
		t.Fatalf("LoadBitbucketPR() error = %v", err)
	}
	if !reflect.DeepEqual(got, pr2) {
		t.Fatalf("LoadBitbucketPR() = %v, want %v", got, pr2)
	}

	// Delete snapshot.
	DeleteBitbucketPR(nil, url)

	got, err = LoadBitbucketPR(nil, url)
	if err != nil {
		t.Fatalf("LoadBitbucketPR() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadBitbucketPR() = %v, want %v", got, nil)
	}
}
