package bitbucket

import (
	"reflect"
	"testing"
)

func TestAccountIDs(t *testing.T) {
	tests := []struct {
		name     string
		accounts []Account
		want     []string
	}{
		{
			name: "empty",
		},
		{
			name: "single",
			accounts: []Account{
				{AccountID: "aaa", Type: "user"},
			},
			want: []string{"aaa"},
		},
		{
			name: "multiple",
			accounts: []Account{
				{AccountID: "aaa", Type: "user"},
				{AccountID: "bbb", Type: "user"},
				{AccountID: "ccc", Type: "user"},
			},
			want: []string{"aaa", "bbb", "ccc"},
		},
		{
			name: "duplicates",
			accounts: []Account{
				{AccountID: "aaa", Type: "user"},
				{AccountID: "bbb", Type: "user"},
				{AccountID: "bbb", Type: "user"},
				{AccountID: "aaa", Type: "user"},
			},
			want: []string{"aaa", "bbb"},
		},
		{
			name: "non-users",
			accounts: []Account{
				{AccountID: "aaa", Type: "user"},
				{AccountID: "team1", Type: "team"},
				{AccountID: "app1", Type: "app_user"},
				{AccountID: "bbb", Type: ""},
			},
			want: []string{"aaa", "bbb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accountIDs(tt.accounts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accountIDs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTMLURL(t *testing.T) {
	tests := []struct {
		name  string
		links map[string]Link
		want  string
	}{
		{
			name: "empty",
		},
		{
			name:  "happy_path",
			links: map[string]Link{"html": {HRef: "http://example.com"}},
			want:  "http://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTMLURL(tt.links); got != tt.want {
				t.Errorf("HTMLURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSwitchPRSnapshot(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	// Initial state.
	snapshot1 := PullRequest{ID: 1, CommitCount: 2, ChangeRequestCount: 3}
	got, err := SwitchPRSnapshot(nil, "url", snapshot1)
	if err != nil {
		t.Fatalf("SwitchSnapshot(1) error = %v", err)
	}
	if got != nil {
		t.Fatalf("SwitchSnapshot(1) = %#v, want %v", got, nil)
	}

	// Update snapshot.
	snapshot2 := PullRequest{ID: 2, CommitCount: 0, ChangeRequestCount: 0}
	got, err = SwitchPRSnapshot(nil, "url", snapshot2)
	if err != nil {
		t.Fatalf("SwitchSnapshot(2) error = %v", err)
	}
	if !reflect.DeepEqual(got, &snapshot1) {
		t.Fatalf("SwitchSnapshot(2) = %#v, want %#v", got, &snapshot1)
	}

	// Update snapshot - test that the counters carry over.
	snapshot3 := PullRequest{ID: 3, CommitCount: 5, ChangeRequestCount: 6}
	got, err = SwitchPRSnapshot(nil, "url", snapshot3)
	if err != nil {
		t.Fatalf("SwitchSnapshot(3) error = %v", err)
	}
	snapshot2.CommitCount = snapshot1.CommitCount
	snapshot2.ChangeRequestCount = snapshot1.ChangeRequestCount
	if !reflect.DeepEqual(got, &snapshot2) {
		t.Fatalf("SwitchSnapshot(3) = %#v, want %#v", got, &snapshot2)
	}

	// Update again - test that the counters can be updated.
	got, err = SwitchPRSnapshot(nil, "url", snapshot3)
	if err != nil {
		t.Fatalf("SwitchSnapshot(4) error = %v", err)
	}
	if !reflect.DeepEqual(got, &snapshot3) {
		t.Fatalf("SwitchSnapshot(4) = %#v, want %#v", got, &snapshot3)
	}
}
