package commands

import (
	"testing"
)

func TestShowDraftsOption(t *testing.T) {
	tests := []struct {
		name    string
		initial bool
		text    string
		want    bool
	}{
		{
			name: "no_change_1",
		},
		{
			name:    "no_change_2",
			initial: true,
			want:    true,
		},
		{
			name: "enable_drafts",
			text: "status @user with drafts",
			want: true,
		},
		{
			name:    "disable_drafts",
			initial: true,
			text:    "status @user no drafts",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := showDraftsOption(tt.initial, tt.text)
			if got != tt.want {
				t.Errorf("showDraftsOption(%v, %q) = %v, want %v", tt.initial, tt.text, got, tt.want)
			}
		})
	}
}

func TestStatusMode(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		wantAuthors   bool
		wantReviewers bool
	}{
		{
			name:        "only_authored_prs",
			text:        "status author @user",
			wantAuthors: true,
		},
		{
			name:          "only_prs_to_review",
			text:          "status reviewer @user",
			wantReviewers: true,
		},
		{
			name:          "both_pr_types",
			text:          "status @user",
			wantAuthors:   true,
			wantReviewers: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAuthors, gotReviewers := statusMode(tt.text)
			if gotAuthors != tt.wantAuthors || gotReviewers != tt.wantReviewers {
				t.Errorf("statusMode(%q) = (%v, %v), want (%v, %v)",
					tt.text, gotAuthors, gotReviewers, tt.wantAuthors, tt.wantReviewers)
			}
		})
	}
}
