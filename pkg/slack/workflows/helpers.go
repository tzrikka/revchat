package workflows

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
)

func commentURL(ctx workflow.Context, ids string) (string, error) {
	url, err := data.SwitchURLAndID(ids)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack message's PR comment URL",
			slog.Any("error", err), slog.String("slack_ids", ids))
		return "", err
	}

	if url == "" {
		logger.From(ctx).Debug("Slack message's PR comment URL is empty", slog.String("slack_ids", ids))
	}

	return url, nil
}

func isRevChatChannel(ctx workflow.Context, channelID string) bool {
	url, _ := commentURL(ctx, channelID)
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
