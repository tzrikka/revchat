package data_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/data"
)

func TestBitbucketBuilds(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	prURL := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initial state.
	got := data.ReadBitbucketBuilds(nil, prURL)
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %#v, want %#v", got, data.PRStatus{})
	}

	// Update build status.
	cs1 := data.CommitStatus{
		Name:  "build1",
		State: "SUCCESS",
		Desc:  "Build passed",
		URL:   "http://build1",
	}
	data.UpdateBitbucketBuilds(nil, prURL, "commit1", "build1", cs1)

	got = data.ReadBitbucketBuilds(nil, prURL)
	want := data.PRStatus{
		CommitHash: "commit1",
		Builds: map[string]data.CommitStatus{
			"build1": cs1,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Update with new commit.
	cs2 := data.CommitStatus{
		Name:  "build2",
		State: "FAILED",
		Desc:  "Build failed",
		URL:   "http://build2",
	}
	data.UpdateBitbucketBuilds(nil, prURL, "commit2", "build2", cs2)

	got = data.ReadBitbucketBuilds(nil, prURL)
	want = data.PRStatus{
		CommitHash: "commit2",
		Builds: map[string]data.CommitStatus{
			"build2": cs2,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Delete builds.
	data.DeleteBitbucketBuilds(nil, prURL)

	got = data.ReadBitbucketBuilds(nil, prURL)
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, data.PRStatus{})
	}
}
