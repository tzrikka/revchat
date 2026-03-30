package workflows

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func (c *Config) isRevChatChannel(ctx workflow.Context, channelID string) bool {
	url, err := c.switchURLAndID(ctx, channelID)
	return err == nil && url != ""
}

func (c *Config) switchURLAndID(ctx workflow.Context, key string) (string, error) {
	url, err := data.SwitchURLAndID(ctx, key)
	if err != nil {
		return "", activities.AlertError(ctx, c.AlertsChannel, "", err, "Key", key)
	}
	return url, nil
}

func selfTriggeredMemberEvent(ctx workflow.Context, auth []eventAuth, event MemberEvent) bool {
	for _, a := range auth {
		if a.IsBot && (a.UserID == event.User || a.UserID == event.Inviter) {
			logger.From(ctx).Debug("ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func selfTriggeredEvent(ctx workflow.Context, auth []eventAuth, userID string) bool {
	for _, a := range auth {
		if a.IsBot && a.UserID == userID {
			logger.From(ctx).Debug("ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}
