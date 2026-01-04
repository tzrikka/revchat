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
				{AccountID: "app1", Type: "app"},
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

func TestSwitchSnapshot(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	// Initial state.
	snapshot1 := PullRequest{ID: 1}
	pr, err := SwitchSnapshot(nil, "url", snapshot1)
	if err != nil {
		t.Fatalf("SwitchSnapshot() error = %v", err)
	}
	if pr != nil {
		t.Fatalf("SwitchSnapshot() = %v, want %v", pr, nil)
	}

	// Replace initial snapshot.
	snapshot2 := PullRequest{ID: 2}
	pr, err = SwitchSnapshot(nil, "url", snapshot2)
	if err != nil {
		t.Fatalf("SwitchSnapshot() error = %v", err)
	}
	if pr == nil {
		t.Fatalf("SwitchSnapshot() = %v, want %v", pr, snapshot2)
	}
	if pr.ID != snapshot1.ID {
		t.Fatalf("SwitchSnapshot() = %v, want %v", pr.ID, snapshot1.ID)
	}
}
