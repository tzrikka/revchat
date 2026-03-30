package internal

import (
	"reflect"
	"testing"
	"time"
)

func TestNormalizeEmailAddresses(t *testing.T) {
	turn := &PRTurns{
		Author:   "AUTHOR",
		FrozenBy: "FrozenBy",
		Reviewers: map[string]bool{
			"Rev1": true,
			"rev2": false,
			"REV3": true,
		},
	}

	normalizeEmailAddresses(turn)

	if turn.Author != "author" {
		t.Fatalf("normalizeEmailAddresses() Author = %q, want %q", turn.Author, "author")
	}
	if turn.FrozenBy != "frozenby" {
		t.Fatalf("normalizeEmailAddresses() FrozenBy = %q, want %q", turn.FrozenBy, "frozenby")
	}
	wantReviewers := map[string]bool{
		"rev1": true,
		"rev2": false,
		"rev3": true,
	}
	if !reflect.DeepEqual(turn.Reviewers, wantReviewers) {
		t.Fatalf("normalizeEmailAddresses() Reviewers = %v, want %v", turn.Reviewers, wantReviewers)
	}
}

func TestListOf(t *testing.T) {
	tests := []struct {
		name string
		pr   map[string]any
		key  string
		want []any
	}{
		{
			name: "missing_key",
			pr:   map[string]any{},
			key:  "key",
			want: []any{},
		},
		{
			name: "good_key",
			pr: map[string]any{
				"key": []any{"a", "b", "c"},
			},
			key:  "key",
			want: []any{"a", "b", "c"},
		},
		{
			name: "wrong_type",
			pr: map[string]any{
				"key": "not a list",
			},
			key:  "key",
			want: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listOf(tt.pr, tt.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("listOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserActivity(t *testing.T) {
	tests := []struct {
		name         string
		detailsMap   any
		wantApproved bool
		wantZero     bool
	}{
		{
			name:       "invalid_participant",
			detailsMap: "not a map",
			wantZero:   true,
		},
		{
			name: "invalid_user",
			detailsMap: map[string]any{
				"user": "not a map",
			},
			wantZero: true,
		},
		{
			name: "missing_approved",
			detailsMap: map[string]any{
				"user": map[string]any{},
			},
			wantZero: true,
		},
		{
			name: "invalid_approved",
			detailsMap: map[string]any{
				"user":     map[string]any{},
				"approved": "not a bool",
			},
			wantZero: true,
		},
		{
			name: "approved",
			detailsMap: map[string]any{
				"user":            map[string]any{},
				"approved":        true,
				"participated_on": time.Now().UTC().Format(time.RFC3339),
			},
			wantApproved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, approved, ts := userActivity(t.Context(), tt.detailsMap)
			if email != "" {
				t.Errorf("userActivity() email = %q, want %q", email, "")
			}
			if approved != tt.wantApproved {
				t.Errorf("userActivity() approved = %v, want %v", approved, tt.wantApproved)
			}
			if ts.IsZero() != tt.wantZero {
				t.Errorf("userActivity() timestamp = %v, want zero: %v", ts, tt.wantZero)
			}
		})
	}
}
