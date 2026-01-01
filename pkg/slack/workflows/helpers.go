package workflows

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
)

func isRevChatChannel(ctx workflow.Context, channelID string) bool {
	url, _ := data.SwitchURLAndID(ctx, channelID)
	return url != ""
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
