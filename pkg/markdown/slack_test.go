package markdown_test

import (
	"strings"
	"testing"

	"github.com/tzrikka/revchat/pkg/markdown"
)

func TestShortenSlackURLs(t *testing.T) {
	shortURL := "<https://example.com/" + strings.Repeat("123456789x", 49) + "|label>" // URL < 512 characters.
	longURL := "<https://example.com/" + strings.Repeat("123456789x", 50) + "|label>"  // URL > 512 characters.

	tests := []struct {
		name       string
		commentURL string
		text       string
		want       string
	}{
		{
			name:       "short_message_with_long_urls_unchanged",
			commentURL: "commentURL",
			text:       strings.Repeat(longURL, 7), // 528 * 7 < 4000.
			want:       strings.Repeat(longURL, 7),
		},
		{
			name:       "long_message_with_short_urls_unchanged",
			commentURL: "commentURL",
			text:       strings.Repeat(shortURL, 8), // 518 * 8 > 4000.
			want:       strings.Repeat(shortURL, 8),
		},
		{
			name:       "long_message_with_long_urls_shortened",
			commentURL: "commentURL",
			text:       strings.Repeat(longURL, 8),              // 528 * 8 > 4000.
			want:       strings.Repeat("<commentURL|label>", 8), //  18 * 8 < 4000.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := markdown.ShortenSlackURLs(tt.commentURL, tt.text); got != tt.want {
				t.Errorf("ShortenSlackURLs() = %q, want %q", got, tt.want)
			}
		})
	}
}
