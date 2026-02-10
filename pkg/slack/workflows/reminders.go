package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

const (
	dateTimeLayout = time.DateOnly + " " + time.Kitchen
)

func (c *Config) RemindersWorkflow(ctx workflow.Context) error {
	startTime := workflow.Now(ctx).UTC().Truncate(time.Minute)

	prs := slack.LoadPRTurns(ctx, true, true, true)
	if len(prs) == 0 {
		return nil
	}

	reminders, err := data.ListReminders(ctx)
	if err != nil {
		return activities.AlertError(ctx, c.AlertsChannel, "", err)
	}

	var aggregatedErr error
	for userID, r := range reminders {
		now, reminderTime, err := reminderTimes(ctx, startTime, userID, r)
		if err != nil {
			err = activities.AlertError(ctx, c.AlertsChannel, "", err, "User", fmt.Sprintf("<@%s>", userID))
			aggregatedErr = errors.Join(aggregatedErr, err)
			continue
		}

		// Send a reminder to the user if their reminder time matches
		// the current time, and there are reminders to be sent to them.
		if userPRs := prs[userID]; reminderTime.Equal(now) && len(userPRs) > 0 {
			logger.From(ctx).Info("sending scheduled Slack reminder to user",
				slog.String("user_id", userID), slog.Int("pr_count", len(userPRs)))
			slices.Sort(userPRs)

			var msg strings.Builder
			msg.WriteString(":bell: This is your scheduled daily reminder to take action on these PRs:")
			singleUser := []string{userID}

			for _, url := range userPRs {
				prDetails := slack.PRDetails(ctx, url, singleUser, true, c.ReportDrafts)

				// If the message becomes too long, split it into multiple chunks,
				// even if the Slack API could technically handle a bit more.
				if msg.Len()+len(prDetails) > 39000 {
					aggregatedErr = errors.Join(aggregatedErr, activities.PostMessage(ctx, userID, msg.String()))
					msg.Reset()
				}

				msg.WriteString(prDetails)
			}

			msg.WriteString("\n\n:information_source: Slash command tips:")
			msg.WriteString("\n  •  `/revchat status` - updated report at any time")
			msg.WriteString("\n  •  `/revchat reminder <time in 12h/24h format>` - change time or timezone")
			msg.WriteString("\n  •  `/revchat who` / `[not] my turn` / `[un]freeze` - only in PR channels")
			msg.WriteString("\n  •  `/revchat explain` - who needs to approve each file, and have they?")

			aggregatedErr = errors.Join(aggregatedErr, activities.PostMessage(ctx, userID, msg.String()))
		}
	}

	return aggregatedErr
}

func reminderTimes(ctx workflow.Context, startTime time.Time, userID, reminder string) (parsed, now time.Time, err error) {
	// Read and parse the daily reminder time for each user.
	kitchenTime, tz, found := strings.Cut(reminder, " ")
	if !found {
		logger.From(ctx).Error("invalid Slack reminder", slog.String("user_id", userID), slog.String("text", reminder))
		err = fmt.Errorf("invalid Slack reminder for Slack user %q: %q", userID, reminder)
		return parsed, now, err
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		logger.From(ctx).Error("invalid timezone in Slack reminder", slog.Any("error", err),
			slog.String("user_id", userID), slog.String("time", reminder), slog.String("tz", tz))
		return parsed, now, err
	}

	now = startTime.In(loc)
	today := now.Format(time.DateOnly)
	rt := fmt.Sprintf("%s %s", today, kitchenTime)
	parsed, err = time.ParseInLocation(dateTimeLayout, rt, loc)
	if err != nil {
		logger.From(ctx).Error("invalid time in Slack reminder", slog.Any("error", err),
			slog.String("user_id", userID), slog.String("date_time", rt))
		return parsed, now, err
	}

	return parsed, now, err
}
