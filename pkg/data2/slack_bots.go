package data2

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data2/internal"
)

func SetSlackBotUserID(ctx workflow.Context, botID, userID string) {
	if ctx == nil { // For unit testing.
		_ = internal.SetSlackBotUserID(context.Background(), botID, userID)
		return
	}

	if err := executeLocalActivity(ctx, internal.SetSlackBotUserID, nil, botID, userID); err != nil {
		logger.From(ctx).Error("failed to store Slack bot's user ID", slog.Any("error", err),
			slog.String("bot_id", botID), slog.String("user_id", userID))
	}
}

func GetSlackBotUserID(ctx workflow.Context, botID string) string {
	if ctx == nil { // For unit testing.
		userID, _ := internal.GetSlackBotUserID(context.Background(), botID)
		return userID
	}

	var userID string
	if err := executeLocalActivity(ctx, internal.GetSlackBotUserID, &userID, botID); err != nil {
		logger.From(ctx).Error("failed to retrieve Slack bot's user ID",
			slog.Any("error", err), slog.String("bot_id", botID))
		return ""
	}

	return userID
}
