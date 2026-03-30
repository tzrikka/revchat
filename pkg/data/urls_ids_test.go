package data_test

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/data"
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
					if err := data.MapURLAndID(nil, tt.url, tt.ids); err != nil {
						t.Fatalf("MapURLAndID() error = %v", err)
					}
				} else {
					data.DeleteURLAndIDMapping(nil, tt.url)
				}
			}

			got, err := data.SwitchURLAndID(nil, "url1")
			if err != nil {
				t.Fatalf("SwitchURLAndID(url1) error = %v", err)
			}
			if got != tt.wantVal1 {
				t.Errorf("SwitchURLAndID(url1) = %q, want %q", got, tt.wantVal1)
			}

			got, err = data.SwitchURLAndID(nil, "ids1")
			if err != nil {
				t.Fatalf("SwitchURLAndID(ids1) error = %v", err)
			}
			if got != tt.wantKey1 {
				t.Errorf("SwitchURLAndID(ids1) = %q, want %q", got, tt.wantKey1)
			}

			got, err = data.SwitchURLAndID(nil, "url2")
			if err != nil {
				t.Fatalf("SwitchURLAndID(url2) error = %v", err)
			}
			if got != tt.wantVal2 {
				t.Errorf("SwitchURLAndID(url2) = %q, want %q", got, tt.wantVal2)
			}

			got, err = data.SwitchURLAndID(nil, "ids2")
			if err != nil {
				t.Fatalf("SwitchURLAndID(ids2) error = %v", err)
			}
			if got != tt.wantKey2 {
				t.Errorf("SwitchURLAndID(ids2) = %q, want %q", got, tt.wantKey2)
			}

			got, err = data.SwitchURLAndID(nil, "url3")
			if err != nil {
				t.Fatalf("SwitchURLAndID(url3) error = %v", err)
			}
			if got != tt.wantVal3 {
				t.Errorf("SwitchURLAndID(url3) = %q, want %q", got, tt.wantVal3)
			}

			got, err = data.SwitchURLAndID(nil, "ids3")
			if err != nil {
				t.Fatalf("SwitchURLAndID(ids3) error = %v", err)
			}
			if got != tt.wantKey3 {
				t.Errorf("SwitchURLAndID(ids3) = %q, want %q", got, tt.wantKey3)
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
			if err := data.MapURLAndID(nil, "https://example.com/pr/12345", "C12345678"); err != nil {
				t.Fatalf("MapURLAndID(PR) error = %v", err)
			}
			if err := data.MapURLAndID(nil, "https://example.com/pr/12345/comment1", "C12345678/commentA"); err != nil {
				t.Fatalf("MapURLAndID(PR comment1) error = %v", err)
			}
			if err := data.MapURLAndID(nil, "https://example.com/pr/12345/comment2", "C12345678/commentA/commentB"); err != nil {
				t.Fatalf("MapURLAndID(PR comment2) error = %v", err)
			}

			data.DeleteURLAndIDMapping(nil, tt.key)

			for _, key := range keys {
				got, err := data.SwitchURLAndID(nil, key)
				if err != nil {
					t.Errorf("SwitchURLAndID(%q) error = %v", key, err)
				}
				if got != "" {
					t.Errorf("SwitchURLAndID(%q) = %q, want empty string", key, got)
				}
			}
		})
	}
}
