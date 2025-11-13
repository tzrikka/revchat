package slack

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

func selfTriggeredMemberEvent(ctx workflow.Context, auth []eventAuth, event MemberEvent) bool {
	for _, a := range auth {
		if a.IsBot && (a.UserID == event.User || a.UserID == event.Inviter) {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func selfTriggeredEvent(ctx workflow.Context, auth []eventAuth, userID string) bool {
	for _, a := range auth {
		if a.IsBot && a.UserID == userID {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func commentURL(ctx workflow.Context, ids string) (string, error) {
	url, err := data.SwitchURLAndID(ids)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack message's PR comment URL", "error", err, "ids", ids)
		return "", err
	}

	if url == "" {
		log.Debug(ctx, "Slack message's PR comment URL is empty", "ids", ids)
	}

	return url, nil
}

func prChannel(ctx workflow.Context, channelID string) bool {
	url, _ := commentURL(ctx, channelID)
	return url != ""
}
