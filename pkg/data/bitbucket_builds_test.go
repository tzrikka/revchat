package data

import (
	"reflect"
	"testing"
)

func TestBitbucketBuilds(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache.Clear()

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initial state.
	got := ReadBitbucketBuilds(nil, url)
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %#v, want %#v", got, PRStatus{})
	}

	// Update build status.
	cs1 := CommitStatus{
		Name:  "build1",
		State: "SUCCESS",
		Desc:  "Build passed",
		URL:   "http://build1",
	}
	UpdateBitbucketBuilds(nil, url, "commit1", "build1", cs1)

	got = ReadBitbucketBuilds(nil, url)
	want := PRStatus{
		CommitHash: "commit1",
		Builds: map[string]CommitStatus{
			"build1": cs1,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Update with new commit.
	cs2 := CommitStatus{
		Name:  "build2",
		State: "FAILED",
		Desc:  "Build failed",
		URL:   "http://build2",
	}
	UpdateBitbucketBuilds(nil, url, "commit2", "build2", cs2)

	got = ReadBitbucketBuilds(nil, url)
	want = PRStatus{
		CommitHash: "commit2",
		Builds: map[string]CommitStatus{
			"build2": cs2,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Delete builds.
	DeleteBitbucketBuilds(nil, url)

	got = ReadBitbucketBuilds(nil, url)
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, PRStatus{})
	}
}
