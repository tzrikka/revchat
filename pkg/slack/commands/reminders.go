package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

// RemindersSyntax is the regular expression that parses the reminders slash command,
// to set the time and timezone for the user's daily reminders.
//
//	/revchat reminder[s] [at] <time in 12h or 24h format> [am|pm|a|p]
var RemindersSyntax = regexp.MustCompile(`^reminders?(\s+at)?\s+([0-9:]+)\s*(am|pm|a|p)?`)

func Reminders(ctx workflow.Context, event SlashCommandEvent) error {
	// Ensure that the calling user is opted-in, i.e. authorized us & allowed to join PR channels.
	_, optedIn, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
		return nil // Not a server error as far as we're concerned.
	}
	if !optedIn {
		PostEphemeralError(ctx, event, "you need to opt-in first.")
		return nil // Not a server error as far as we're concerned.
	}

	matches := RemindersSyntax.FindStringSubmatch(event.Text)
	if len(matches) < 3 {
		logger.From(ctx).Error("failed to parse reminders slash command - regex mismatch", slog.String("text", event.Text))
		PostEphemeralError(ctx, event, "unexpected internal error while parsing command.")
		return errors.New("failed to parse reminders command - regex mismatch")
	}

	amPm := ""
	if len(matches) > 3 {
		amPm = matches[3]
	}

	kitchenTime, err := slack.NormalizeTime(matches[2], amPm)
	if err != nil {
		logger.From(ctx).Warn("failed to parse time in reminders slash command", slog.Any("error", err))
		PostEphemeralError(ctx, event, err.Error())
		return nil // Not a server error as far as we're concerned.
	}

	if kt, _ := time.Parse(time.Kitchen, kitchenTime); kt.Minute() != 0 && kt.Minute() != 30 {
		logger.From(ctx).Warn("uncommon reminder time requested",
			slog.String("user_id", event.UserID), slog.String("time", kitchenTime))
		PostEphemeralError(ctx, event, "please specify a time on the hour or half-hour.")
		return nil // Not a server error as far as we're concerned.
	}

	return SetReminder(ctx, event, kitchenTime, false)
}

func SetReminder(ctx workflow.Context, event SlashCommandEvent, t string, quiet bool) error {
	user, err := tslack.UsersInfo(ctx, event.UserID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", event.UserID))
		PostEphemeralError(ctx, event, "failed to retrieve Slack user info.")
		return err
	}

	if user.TZ == "" {
		logger.From(ctx).Warn("Slack user has no timezone in their profile", slog.String("user_id", event.UserID))
		PostEphemeralError(ctx, event, "please set a timezone in your Slack profile preferences first.")
		return nil // Not a server error as far as we're concerned.
	}

	if _, err := time.LoadLocation(user.TZ); err != nil {
		logger.From(ctx).Error("unrecognized user timezone", slog.Any("error", err),
			slog.String("user_id", event.UserID), slog.String("tz", user.TZ))
		PostEphemeralError(ctx, event, fmt.Sprintf("your Slack timezone is unrecognized: `%s`", user.TZ))
		return err
	}

	if err := data.SetReminder(ctx, event.UserID, t, user.TZ); err != nil {
		logger.From(ctx).Error("failed to store user reminder time", slog.Any("error", err),
			slog.String("user_id", event.UserID), slog.String("time", t), slog.String("zone", user.TZ))
		PostEphemeralError(ctx, event, "failed to write internal data about you.")
		return err
	}

	if !quiet {
		t = fmt.Sprintf("%s %s", t[:len(t)-2], t[len(t)-2:]) // Insert space before AM/PM suffix.
		msg := fmt.Sprintf(":alarm_clock: Your daily reminder time is set to *%s* _(%s)_ on weekdays.", t, user.TZ)
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	return nil
}
