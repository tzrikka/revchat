package commands

import (
	"testing"
)

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
