package data2

import (
	"reflect"
	"testing"
)

func TestBitbucketBuilds(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	prURL := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initial state.
	got := ReadBitbucketBuilds(nil, prURL)
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
	UpdateBitbucketBuilds(nil, prURL, "commit1", "build1", cs1)

	got = ReadBitbucketBuilds(nil, prURL)
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
	UpdateBitbucketBuilds(nil, prURL, "commit2", "build2", cs2)

	got = ReadBitbucketBuilds(nil, prURL)
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
	DeleteBitbucketBuilds(nil, prURL)

	got = ReadBitbucketBuilds(nil, prURL)
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, PRStatus{})
	}
}
