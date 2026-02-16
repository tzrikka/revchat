package markdown

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// These global variables don't require synchronization.
// Worst case, we overwrite with the same value once.
var (
	cachedSlackBaseURL  = ""
	cachedSlackChannels = cache.New[string](10*time.Minute, cache.DefaultCleanupInterval)

	slackURLPattern = regexp.MustCompile(`<([^|]+)\|([^>]+)>`)
)

func slackBaseURL(ctx workflow.Context) string {
	if cachedSlackBaseURL != "" {
		return cachedSlackBaseURL
	}

	resp, err := slack.AuthTest(ctx)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack auth info", slog.Any("error", err))
		return ""
	}

	cachedSlackBaseURL = resp.URL
	return cachedSlackBaseURL
}

func slackChannelIDToName(ctx workflow.Context, id string) string {
	if name, ok := cachedSlackChannels.Get(id); ok {
		return name
	}

	channel, err := slack.ConversationsInfo(ctx, id, false, false)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack channel info",
			slog.Any("error", err), slog.String("channel_id", id))
		return ""
	}

	name, ok := channel["name"].(string)
	if !ok {
		logger.From(ctx).Error("Slack channel 'name' field missing or not a string", slog.String("channel_id", id))
		return ""
	}

	cachedSlackChannels.Set(id, name, cache.DefaultExpiration)
	return name
}

// ShortenSlackURLs ensures that updates of Slack messages won't fail due to a "msg_too_long" error
// (API limit: text length <= 4000 characters). When the text approaches this limit, we look for long
// URLs (512+ characters) that are masked by a label, and replace them with the PR comment's URL, so
// they're still accessible with an extra click but don't take up so much space in the message itself.
func ShortenSlackURLs(commentURL, text string) string {
	if len(text) <= 3900 {
		return text
	}

	for _, url := range slackURLPattern.FindAllStringSubmatch(text, -1) {
		if len(url) == 3 && len(url[1]) >= 512 {
			text = strings.ReplaceAll(text, url[0], fmt.Sprintf("<%s|%s>", commentURL, url[2]))
		}
	}

	return text
}
