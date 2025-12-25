package bitbucket

import (
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

var linkifyPattern = regexp.MustCompile(`[A-Z]{2,}-\d+`)

// LinkifyTitle finds IDs in the text and tries to linkify them based on the configured issue
// trackers. If no IDs are found, or none are recognized, it returns the input text unchanged.
func LinkifyTitle(ctx workflow.Context, cfg map[string]string, title string) string {
	for _, id := range linkifyPattern.FindAllString(title, -1) {
		if url := linkifyID(ctx, cfg, id); url != "" {
			title = strings.Replace(title, id, fmt.Sprintf("<%s|%s>", url, id), 1)
		}
	}
	return title
}

// linkifyID can recognize specific case-sensitive keys, as well as a generic "default" key.
func linkifyID(ctx workflow.Context, cfg map[string]string, id string) string {
	linkKey, _, _ := strings.Cut(id, "-")
	if baseURL, found := cfg[linkKey]; found {
		return buildLink(ctx, baseURL, id)
	}

	if baseURL, found := cfg["default"]; found {
		return buildLink(ctx, baseURL, id)
	}

	return ""
}

func buildLink(ctx workflow.Context, base, id string) string {
	link, err := url.JoinPath(base, id)
	if err != nil {
		logger.From(ctx).Warn("failed to join URL paths", slog.Any("error", err),
			slog.String("base_url", base), slog.String("key_id", id))
		return ""
	}
	return link
}
