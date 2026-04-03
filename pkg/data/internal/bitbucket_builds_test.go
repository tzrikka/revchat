package internal_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/data/internal"
)

func TestBitbucketBuilds(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	prURL := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initial state.
	got, err := internal.ReadBitbucketBuilds(t.Context(), prURL)
	if err != nil {
		t.Fatalf("ReadBitbucketBuilds() error = %v", err)
	}

	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %#v, want %#v", got, &internal.PRStatus{})
	}

	// Update build status.
	cs1 := internal.CommitStatus{
		Name:  "build1",
		State: "SUCCESS",
		Desc:  "Build passed",
		URL:   "http://build1",
	}
	if err := internal.UpdateBitbucketBuilds(t.Context(), prURL, "commit1", "build1", cs1); err != nil {
		t.Fatalf("UpdateBitbucketBuilds() error = %v", err)
	}

	got, err = internal.ReadBitbucketBuilds(t.Context(), prURL)
	if err != nil {
		t.Fatalf("ReadBitbucketBuilds() error = %v", err)
	}
	want := &internal.PRStatus{
		CommitHash: "commit1",
		Builds: map[string]internal.CommitStatus{
			"build1": cs1,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Update with new commit.
	cs2 := internal.CommitStatus{
		Name:  "build2",
		State: "FAILED",
		Desc:  "Build failed",
		URL:   "http://build2",
	}
	if err := internal.UpdateBitbucketBuilds(t.Context(), prURL, "commit2", "build2", cs2); err != nil {
		t.Fatalf("UpdateBitbucketBuilds() error = %v", err)
	}

	got, err = internal.ReadBitbucketBuilds(t.Context(), prURL)
	if err != nil {
		t.Fatalf("ReadBitbucketBuilds() error = %v", err)
	}
	want = &internal.PRStatus{
		CommitHash: "commit2",
		Builds: map[string]internal.CommitStatus{
			"build2": cs2,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, want)
	}

	// Delete builds.
	if err := internal.DeleteGenericPRFile(t.Context(), prURL+internal.BuildsFileSuffix); err != nil {
		t.Fatalf("DeleteGenericPRFile() error = %v", err)
	}

	got, err = internal.ReadBitbucketBuilds(t.Context(), prURL)
	if err != nil {
		t.Fatalf("ReadBitbucketBuilds() error = %v", err)
	}
	if got.Builds != nil {
		t.Fatalf("ReadBitbucketBuilds() = %v, want %v", got, &internal.PRStatus{})
	}
}
