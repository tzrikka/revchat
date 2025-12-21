package markdown

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// These global variables don't require synchronization.
// Worst case, we overwrite with the same value once.
var (
	cachedSlackBaseURL  = ""
	cachedSlackChannels = map[string]string{}
)

func slackBaseURL(ctx workflow.Context) string {
	if cachedSlackBaseURL != "" {
		return cachedSlackBaseURL
	}

	resp, err := slack.AuthTest(ctx)
	if err != nil {
		logger.Error(ctx, "failed to retrieve Slack auth info", err)
		return ""
	}

	cachedSlackBaseURL = resp.URL
	return cachedSlackBaseURL
}

func slackChannelIDToName(ctx workflow.Context, id string) string {
	if name, ok := cachedSlackChannels[id]; ok {
		return name
	}

	channel, err := slack.ConversationsInfo(ctx, id, false, false)
	if err != nil {
		logger.Error(ctx, "failed to retrieve Slack channel info", err, slog.String("channel_id", id))
		return ""
	}

	name, ok := channel["name"].(string)
	if !ok {
		logger.Error(ctx, "Slack channel 'name' field missing or not a string", nil, slog.String("channel_id", id))
		return ""
	}

	cachedSlackChannels[id] = name
	return name
}
