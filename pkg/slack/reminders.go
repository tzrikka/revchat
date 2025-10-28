package slack

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

const (
	dateTimeLayout = time.DateOnly + " " + time.Kitchen
)

func (c *Config) remindersWorkflow(ctx workflow.Context) error {
	startTime := workflow.Now(ctx).UTC().Truncate(time.Minute)

	rs, err := data.ListReminders()
	if err != nil {
		return err
	}

	for userID, r := range rs {
		// Read and parse the daily reminder time for each user.
		kitchenTime, tz, found := strings.Cut(r, " ")
		if !found {
			log.Error(ctx, "invalid Slack reminder", "user_id", userID, "text", r)
			continue
		}

		loc, err := time.LoadLocation(tz)
		if err != nil {
			log.Error(ctx, "invalid timezone in Slack reminder", "error", err, "user_id", userID, "time", r, "tz", tz)
			continue
		}

		now := startTime.In(loc)
		today := now.Format(time.DateOnly)
		reminderTime := fmt.Sprintf("%s %s", today, kitchenTime)
		t, err := time.ParseInLocation(dateTimeLayout, reminderTime, loc)
		if err != nil {
			log.Error(ctx, "invalid time in Slack reminder", "error", err, "user_id", userID, "date_time", reminderTime)
			continue
		}

		// Send a reminder to the user if their reminder time matches
		// the current time, and there are reminders to be sent.
		if t.Equal(now) {
			log.Info(ctx, "sending scheduled Slack reminder to user", "user_id", userID)
			_, _ = PostMessage(ctx, userID, "Boo!")
		}
	}

	return nil
}
