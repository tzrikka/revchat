package internal_test

import (
	"slices"
	"testing"

	"github.com/tzrikka/revchat/pkg/data/internal"
)

func TestURLsIDs(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	tests := []struct {
		name     string
		url      string
		ids      string
		wantKey1 string
		wantVal1 string
		wantKey2 string
		wantVal2 string
		wantKey3 string
		wantVal3 string
	}{
		{
			name: "initial_state",
		},
		{
			name:     "set_and_get",
			url:      "url1",
			ids:      "ids1",
			wantKey1: "url1",
			wantVal1: "ids1",
		},
		{
			name:     "another_set_and_get",
			url:      "url2",
			ids:      "ids2",
			wantKey1: "url1",
			wantVal1: "ids1",
			wantKey2: "url2",
			wantVal2: "ids2",
		},
		{
			name:     "update_and_get",
			url:      "url1",
			ids:      "ids3",
			wantKey1: "url1",
			wantVal1: "ids3",
			wantKey2: "url2",
			wantVal2: "ids2",
			wantKey3: "url1",
		},
		{
			name:     "delete_and_get",
			url:      "url1",
			wantKey2: "url2",
			wantVal2: "ids2",
		},
		{
			name: "another_delete_and_get",
			url:  "url2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.url != "" {
				if tt.ids != "" {
					if err := internal.SetURLAndIDMapping(t.Context(), tt.url, tt.ids); err != nil {
						t.Fatalf("SetURLAndIDMapping() error = %v", err)
					}
				} else {
					if err := internal.DelURLAndIDMapping(t.Context(), tt.url); err != nil {
						t.Fatalf("DelURLAndIDMapping(%q) error = %v", tt.url, err)
					}
				}
			}

			got, err := internal.GetURLAndIDMapping(t.Context(), "url1")
			if err != nil {
				t.Fatalf("GetURLAndIDMapping(url1) error = %v", err)
			}
			if got != tt.wantVal1 {
				t.Errorf("GetURLAndIDMapping(url1) = %q, want %q", got, tt.wantVal1)
			}

			got, err = internal.GetURLAndIDMapping(t.Context(), "ids1")
			if err != nil {
				t.Fatalf("GetURLAndIDMapping(ids1) error = %v", err)
			}
			if got != tt.wantKey1 {
				t.Errorf("GetURLAndIDMapping(ids1) = %q, want %q", got, tt.wantKey1)
			}

			got, err = internal.GetURLAndIDMapping(t.Context(), "url2")
			if err != nil {
				t.Fatalf("GetURLAndIDMapping(url2) error = %v", err)
			}
			if got != tt.wantVal2 {
				t.Errorf("GetURLAndIDMapping(url2) = %q, want %q", got, tt.wantVal2)
			}

			got, err = internal.GetURLAndIDMapping(t.Context(), "ids2")
			if err != nil {
				t.Fatalf("GetURLAndIDMapping(ids2) error = %v", err)
			}
			if got != tt.wantKey2 {
				t.Errorf("GetURLAndIDMapping(ids2) = %q, want %q", got, tt.wantKey2)
			}

			got, err = internal.GetURLAndIDMapping(t.Context(), "url3")
			if err != nil {
				t.Fatalf("GetURLAndIDMapping(url3) error = %v", err)
			}
			if got != tt.wantVal3 {
				t.Errorf("GetURLAndIDMapping(url3) = %q, want %q", got, tt.wantVal3)
			}

			got, err = internal.GetURLAndIDMapping(t.Context(), "ids3")
			if err != nil {
				t.Fatalf("GetURLAndIDMapping(ids3) error = %v", err)
			}
			if got != tt.wantKey3 {
				t.Errorf("GetURLAndIDMapping(ids3) = %q, want %q", got, tt.wantKey3)
			}
		})
	}
}

func TestDeleteURLAndIDMapping(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	tests := []struct {
		name string
		key  string
	}{
		{
			name: "delete_pr_url",
			key:  "https://example.com/pr/12345",
		},
		{
			name: "delete_slack_channel",
			key:  "C12345678",
		},
	}

	keys := []string{
		"https://example.com/pr/12345",
		"C12345678",
		"https://example.com/pr/12345/comment1",
		"C12345678/commentA",
		"https://example.com/pr/12345/comment2",
		"C12345678/commentA/commentB",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := internal.SetURLAndIDMapping(t.Context(), "https://example.com/pr/12345", "C12345678"); err != nil {
				t.Fatalf("SetURLAndIDMapping(PR) error = %v", err)
			}
			if err := internal.SetURLAndIDMapping(t.Context(), "https://example.com/pr/12345/comment1", "C12345678/commentA"); err != nil {
				t.Fatalf("SetURLAndIDMapping(PR comment1) error = %v", err)
			}
			if err := internal.SetURLAndIDMapping(t.Context(), "https://example.com/pr/12345/comment2", "C12345678/commentA/commentB"); err != nil {
				t.Fatalf("SetURLAndIDMapping(PR comment2) error = %v", err)
			}

			if err := internal.DelURLAndIDMapping(t.Context(), tt.key); err != nil {
				t.Fatalf("DelURLAndIDMapping(%q) error = %v", tt.key, err)
			}

			for _, key := range keys {
				got, err := internal.GetURLAndIDMapping(t.Context(), key)
				if err != nil {
					t.Errorf("GetURLAndIDMapping(%q) error = %v", key, err)
				}
				if got != "" {
					t.Errorf("GetURLAndIDMapping(%q) = %q, want empty string", key, got)
				}
			}
		})
	}
}

func TestReadAllURLsOrChannels(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	if err := internal.SetURLAndIDMapping(t.Context(), "https://example.com/foo/bar/pull/123", "C123"); err != nil {
		t.Fatalf("SetURLAndIDMapping() error = %v", err)
	}
	if err := internal.SetURLAndIDMapping(t.Context(), "https://example.com/foo/bar/pull/123/comment1", "C123/456"); err != nil {
		t.Fatalf("SetURLAndIDMapping() error = %v", err)
	}
	if err := internal.SetURLAndIDMapping(t.Context(), "https://example.com/foo/bar/pull/456/comment2", "C456/789"); err != nil {
		t.Fatalf("SetURLAndIDMapping() error = %v", err)
	}

	tests := []struct {
		name string
		what string
		want []string
	}{
		{
			name: "urls",
			what: internal.URLs,
			want: []string{
				"https://example.com/foo/bar/pull/123",
				"https://example.com/foo/bar/pull/456",
			},
		},
		{
			name: "channels",
			what: internal.Channels,
			want: []string{
				"C123",
				"C456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := internal.ReadAllURLsOrChannels(t.Context(), tt.what)
			if err != nil {
				t.Fatalf("ReadAllURLsOrChannels(%q) error = %v", tt.what, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ReadAllURLsOrChannels(%q) = %q, want %q", tt.what, got, tt.want)
			}
		})
	}
}
