package markdown

import (
	"log/slog"
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
	cachedSlackChannels = cache.New(10*time.Minute, cache.DefaultCleanupInterval)
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
