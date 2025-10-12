package markdown

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
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

	resp, err := slack.AuthTestActivity(ctx)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack auth info", "error", err)
		return ""
	}

	cachedSlackBaseURL = resp.URL
	return cachedSlackBaseURL
}

func slackChannelIDToName(ctx workflow.Context, id string) string {
	if name, ok := cachedSlackChannels[id]; ok {
		return name
	}

	channel, err := slack.ConversationsInfoActivity(ctx, id, false, false)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack channel info", "error", err, "channel_id", id)
		return ""
	}

	name, ok := channel["name"].(string)
	if !ok {
		log.Error(ctx, "Slack channel 'name' field missing or not a string", "channel_id", id)
		return ""
	}

	cachedSlackChannels[id] = name
	return name
}
