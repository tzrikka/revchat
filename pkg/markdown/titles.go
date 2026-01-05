package markdown

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
	linkifyPattern = regexp.MustCompile(`([A-Z]{2,})-\d+`)
	baseURLPattern = regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+)/(pull|pull-requests)/`)
	prIDPattern    = regexp.MustCompile(`(([\w-]+/)?([\w-]+))?#(\d+)`)
)

// LinkifyTitle finds IDs in the given title of a pull/merge request and tries to replace them with
// Slack links: in the same repository, a nearby one, or based on RevChat's configuration of issue
// trackers. If no IDs are found, or none are recognized, it returns the input text unchanged.
func LinkifyTitle(ctx workflow.Context, cfg map[string]string, prURL, title string) string {
	done := map[string]bool{}

	for _, id := range linkifyPattern.FindAllStringSubmatch(title, -1) {
		if link := linkifyID(ctx, cfg, id); link != "" && !done[id[0]] {
			title = strings.ReplaceAll(title, id[0], fmt.Sprintf("<%s|%s>", link, id[0]))
			done[id[0]] = true
		}
	}

	baseURL := baseURLPattern.FindStringSubmatch(prURL)
	if len(baseURL) < 5 {
		return title
	}
	for _, id := range prIDPattern.FindAllStringSubmatch(title, -1) {
		if len(id) == 5 && !done[id[0]] {
			title = strings.ReplaceAll(title, id[0], linkifyPR(baseURL, id))
			done[id[0]] = true
		}
	}

	return title
}

// linkifyID recognizes specific case-sensitive ID keys, but can also use a generic "default" ID key.
func linkifyID(ctx workflow.Context, cfg map[string]string, id []string) string {
	if len(id) < 2 {
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

// linkifyPR recognizes "[[org/]repo]#123" as PR references in/near the current repo.
// Note that in GitHub's case this works for repo issues as well as PRs.
func linkifyPR(baseURL, id []string) string {
	if id[2] == "" {
		id[2] = baseURL[2]
	} else {
		id[2], _ = strings.CutSuffix(id[2], "/")
	}

	if id[3] == "" {
		id[3] = baseURL[3]
	}

	return fmt.Sprintf("<https://%s/%s/%s/%s/%s|%s>", baseURL[1], id[2], id[3], baseURL[4], id[4], id[0])
}
