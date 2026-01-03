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

var (
	linkifyPattern = regexp.MustCompile(`([A-Z]{2,})-(\d+)`)
	baseURLPattern = regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+)/pull-requests/`)
	prIDPattern    = regexp.MustCompile(`(([\w-]+/)?([\w-]+))?#(\d+)`)
)

// LinkifyTitle finds IDs in the text and tries to linkify them - in the same repo, a nearby one, or based on the
// configured issue trackers. If no IDs are found, or none are recognized, it returns the input text unchanged.
func LinkifyTitle(ctx workflow.Context, cfg map[string]string, prURL, title string) string {
	done := map[string]bool{}

	for _, id := range linkifyPattern.FindAllStringSubmatch(title, -1) {
		if url := linkifyID(ctx, cfg, id); url != "" && !done[id[0]] {
			title = strings.ReplaceAll(title, id[0], fmt.Sprintf("<%s|%s>", url, id[0]))
			done[id[0]] = true
		}
	}

	baseURL := baseURLPattern.FindAllStringSubmatch(prURL, -1)
	if len(baseURL) == 0 || len(baseURL[0]) < 4 {
		return title
	}
	for _, id := range prIDPattern.FindAllStringSubmatch(title, -1) {
		if len(id) < 5 || done[id[0]] {
			continue
		}
		title = strings.ReplaceAll(title, id[0], linkifyPR(baseURL[0], id))
		done[id[0]] = true
	}

	return title
}

// linkifyID recognizes specific case-sensitive ID keys, but can also use a generic "default" ID key.
func linkifyID(ctx workflow.Context, cfg map[string]string, id []string) string {
	if len(id) < 3 {
		return ""
	}

	if baseURL, found := cfg[id[1]]; found {
		return buildLink(ctx, baseURL, id[0])
	}
	if baseURL, found := cfg["default"]; found {
		return buildLink(ctx, baseURL, id[0])
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

// linkifyPR recognizes "[[proj/]repo]#123" as PR references in/near the current repo.
func linkifyPR(baseURL, id []string) string {
	if id[2] == "" {
		id[2] = baseURL[2]
	} else {
		id[2], _ = strings.CutSuffix(id[2], "/")
	}

	if id[3] == "" {
		id[3] = baseURL[3]
	}

	return fmt.Sprintf("<https://%s/%s/%s/pull-requests/%s|%s>", baseURL[1], id[2], id[3], id[4], id[0])
}
