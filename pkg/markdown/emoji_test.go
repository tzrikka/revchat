package markdown_test

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/markdown"
)

func TestBitbucketEmojiConversions(t *testing.T) {
	text := "Hello :robot: world :rofl:"
	want := "Hello :robot_face: world :rolling_on_the_floor_laughing:"

	got1 := markdown.BitbucketToSlackEmoji(text)
	if got1 != want {
		t.Fatalf("BitbucketToSlackEmoji(%q) = %q, want %q", text, got1, want)
	}

	// Idempotency check in one direction.
	if got2 := markdown.BitbucketToSlackEmoji(got1); got1 != got2 {
		t.Errorf("BitbucketToSlackEmoji is not idempotent: %q --> %q", got1, got2)
	}

	got3 := markdown.SlackToBitbucketEmoji(got1)
	if got3 != text {
		t.Fatalf("SlackToBitbucketEmoji(%q) = %q, want %q", got1, got3, text)
	}

	// Idempotency check in the other direction.
	if got4 := markdown.SlackToBitbucketEmoji(got3); got3 != got4 {
		t.Errorf("SlackToBitbucketEmoji is not idempotent: %q --> %q", got3, got4)
	}
}

func TestGitHubEmojiConversions(t *testing.T) {
	text := "Hello :robot: world :rofl:"
	want := "Hello :robot_face: world :rolling_on_the_floor_laughing:"

	got1 := markdown.GitHubToSlackEmoji(text)
	if got1 != want {
		t.Fatalf("GitHubToSlackEmoji(%q) = %q, want %q", text, got1, want)
	}

	// Idempotency check in one direction.
	if got2 := markdown.GitHubToSlackEmoji(got1); got1 != got2 {
		t.Errorf("GitHubToSlackEmoji is not idempotent: %q --> %q", got1, got2)
	}

	got3 := markdown.SlackToGitHubEmoji(got1)
	if got3 != text {
		t.Fatalf("SlackToGitHubEmoji(%q) = %q, want %q", got1, got3, text)
	}

	// Idempotency check in the other direction.
	if got4 := markdown.SlackToGitHubEmoji(got3); got3 != got4 {
		t.Errorf("SlackToGitHubEmoji is not idempotent: %q --> %q", got3, got4)
	}
}

func TestGitHubOK(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "ok_person",
			text: ":ok_person:",
			want: ":ok_woman:",
		},
		{
			name: "ok_woman",
			text: ":ok_woman:",
			want: ":woman-gesturing-ok:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdown.GitHubToSlackEmoji(tt.text)
			if got != tt.want {
				t.Fatalf("GitHubToSlackEmoji(%q) = %q, want %q", tt.text, got, tt.want)
			}

			got2 := markdown.SlackToGitHubEmoji(got)
			if got2 != tt.text {
				t.Fatalf("SlackToGitHubEmoji(%q) = %q, want %q", got, got2, tt.text)
			}
		})
	}
}

func TestSkinTones(t *testing.T) {
	text := ":wave::skin-tone-3:"
	want := ":wave:"

	if got := markdown.SlackToBitbucketEmoji(text); got != want {
		t.Fatalf("SlackToBitbucketEmoji(%q) = %q, want %q", text, got, want)
	}

	if got := markdown.SlackToGitHubEmoji(text); got != want {
		t.Fatalf("SlackToGitHubEmoji(%q) = %q, want %q", text, got, want)
	}
}
