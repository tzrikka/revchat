package data2_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/data2"
)

func TestBitbucketBuilds(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	prURL := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initial state.
	got := data2.ReadBitbucketBuilds(nil, prURL)
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %#v, want %#v", got, data2.PRStatus{})
	}

	// Update build status.
	cs1 := data2.CommitStatus{
		Name:  "build1",
		State: "SUCCESS",
		Desc:  "Build passed",
		URL:   "http://build1",
	}
	data2.UpdateBitbucketBuilds(nil, prURL, "commit1", "build1", cs1)

	got = data2.ReadBitbucketBuilds(nil, prURL)
	want := data2.PRStatus{
		CommitHash: "commit1",
		Builds: map[string]data2.CommitStatus{
			"build1": cs1,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Update with new commit.
	cs2 := data2.CommitStatus{
		Name:  "build2",
		State: "FAILED",
		Desc:  "Build failed",
		URL:   "http://build2",
	}
	data2.UpdateBitbucketBuilds(nil, prURL, "commit2", "build2", cs2)

	got = data2.ReadBitbucketBuilds(nil, prURL)
	want = data2.PRStatus{
		CommitHash: "commit2",
		Builds: map[string]data2.CommitStatus{
			"build2": cs2,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Delete builds.
	data2.DeleteBitbucketBuilds(nil, prURL)

	got = data2.ReadBitbucketBuilds(nil, prURL)
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, data2.PRStatus{})
	}
}
