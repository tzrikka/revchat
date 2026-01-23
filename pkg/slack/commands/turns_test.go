package commands

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/data"
)

func TestWhoseTurnText(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	tests := []struct {
		name   string
		emails []string
		user   data.User
		tweak  string
		want   string
	}{
		{
			name:   "author_only_with_tweak",
			emails: []string{"author@example.com"},
			user:   data.User{Email: "author@example.com"},
			tweak:  " now",
			want:   ":eyes: I think it's now *your* turn to review this PR.",
		},
		{
			name:   "author_and_1_reviewer",
			emails: []string{"author@example.com", "reviewer@example.com"},
			user:   data.User{Email: "author@example.com"},
			want:   ":eyes: I think it's *your* turn - along with reviewer@example.com - to review this PR.",
		},
		{
			name:   "author_and_3_reviewers",
			emails: []string{"reviewer1@example.com", "author@example.com", "reviewer2@example.com", "reviewer3@example.com"},
			user:   data.User{Email: "author@example.com"},
			want:   ":eyes: I think it's *your* turn - along with reviewer1@example.com, reviewer2@example.com, reviewer3@example.com - to review this PR.",
		},
		{
			name:   "1_reviewer",
			emails: []string{"reviewer@example.com"},
			user:   data.User{Email: "author@example.com"},
			want:   "I think it's the turn of reviewer@example.com to review this PR.",
		},
		{
			name:   "2_reviewers",
			emails: []string{"reviewer1@example.com", "reviewer2@example.com"},
			user:   data.User{Email: "author@example.com"},
			want:   "I think it's the turn of reviewer1@example.com, reviewer2@example.com to review this PR.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := whoseTurnText(nil, tt.emails, tt.user, tt.tweak); got != tt.want {
				t.Errorf("whoseTurnText() = %q, want %q", got, tt.want)
			}
		})
	}
}
