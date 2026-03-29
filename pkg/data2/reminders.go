package data2

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data2/internal"
)

func SetScheduledUserReminder(ctx workflow.Context, userID, kitchenTime, tz string) error {
	if ctx == nil { // For unit testing.
		return internal.SetReminder(context.Background(), userID, kitchenTime, tz)
	}

	if err := executeLocalActivity(ctx, internal.SetReminder, nil, userID, kitchenTime, tz); err != nil {
		logger.From(ctx).Error("failed to set user's scheduled reminder", slog.Any("error", err),
			slog.String("user_id", userID), slog.String("time", kitchenTime), slog.String("zone", tz))
		return err
	}

	return nil
}

func DeleteScheduledUserReminder(ctx workflow.Context, userID string) {
	if ctx == nil { // For unit testing.
		_ = internal.DeleteReminder(context.Background(), userID)
		return
	}

	if err := executeLocalActivity(ctx, internal.DeleteReminder, nil, userID); err != nil {
		logger.From(ctx).Error("failed to delete user's scheduled reminder",
			slog.Any("error", err), slog.String("user_id", userID))
	}
}

func ListScheduledUserReminders(ctx workflow.Context) (map[string]string, error) {
	if ctx == nil { // For unit testing.
		return internal.ListReminders(context.Background())
	}

	var reminders map[string]string
	if err := executeLocalActivity(ctx, internal.ListReminders, &reminders); err != nil {
		logger.From(ctx).Error("failed to list scheduled user reminders", slog.Any("error", err))
		return nil, err
	}

	return reminders, nil
}
