package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/slack/commands"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

const (
	DefaultReminderTime = "8:00AM"
)

// OptInSlashCommand directs the user to authorize RevChat to act on their behalf in the configured SCM (Bitbucket
// or GitHub) using an OAuth 2.0 3-legged flow, and saves or updates their details in RevChat's user database.
//
// This function is in the "workflows" package instead of "commands" because it requires the
// Thrippy details in [Config], and starts a child workflow to wait for the OAuth flow to complete.
func (c *Config) OptInSlashCommand(ctx workflow.Context, event commands.SlashCommandEvent) error {
	user, optedIn, err := commands.UserDetails(ctx, event, event.UserID)
	if err != nil {
		return activities.AlertError(ctx, c.AlertsChannel, "user details", err, "User", fmt.Sprintf("<@%s>", event.UserID))
	}
	if optedIn {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":bell: You're already opted in.")
	}

	info, err := slack.UsersInfo(ctx, event.UserID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", event.UserID))
		commands.PostEphemeralError(ctx, event, "failed to retrieve your user info from Slack.")
		return activities.AlertError(ctx, c.AlertsChannel, "user info", err, "User", fmt.Sprintf("<@%s>", event.UserID))
	}

	// Ensure the user's basic details are set even if they're unrecognized.
	user.RealName = info.RealName
	user.SlackID = event.UserID
	user.Email = strings.ToLower(info.Profile.Email)
	if info.IsBot {
		user.Email = "bot"
	}

	scm := "GitHub"
	if c.BitbucketWorkspace != "" {
		scm = "Bitbucket"
	}

	thrippyID, nonce, err := c.createThrippyLink(ctx)
	if err != nil {
		logger.From(ctx).Error("failed to create Thrippy link for Slack user", slog.Any("error", err))
		commands.PostEphemeralError(ctx, event, "internal authorization failure.")
		return activities.AlertError(ctx, c.AlertsChannel, "create Thrippy link", err, "User", fmt.Sprintf("<@%s>", event.UserID))
	}

	msg := ":point_right: <https://%s/start?id=%s&nonce=%s|Click here> to authorize RevChat to act on your behalf in %s."
	msg = fmt.Sprintf(msg, c.ThrippyHTTPAddress, thrippyID, nonce, scm)
	if err := activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg); err != nil {
		logger.From(ctx).Error("failed to post ephemeral opt-in message in Slack", slog.Any("error", err))
		err = errors.Join(err, c.deleteThrippyLink(ctx, thrippyID))
		return activities.AlertError(ctx, c.AlertsChannel, "post OAuth URL", err, "User", fmt.Sprintf("<@%s>", event.UserID))
	}

	err = c.waitForThrippyLinkCreds(ctx, thrippyID)
	if err != nil {
		err = errors.Join(err, c.deleteThrippyLink(ctx, thrippyID))

		if err.Error() == errLinkAuthzTimeout { // For some reason [errors.Is] doesn't work across Temporal?
			logger.From(ctx).Warn("user did not complete Thrippy OAuth flow in time", slog.String("email", user.Email))
			commands.PostEphemeralError(ctx, event, scm+" authorization timed out - please try opting in again.")
			return nil // Not a server error as far as we're concerned.
		}

		logger.From(ctx).Error("failed to authorize user", slog.Any("error", err), slog.String("email", user.Email))
		commands.PostEphemeralError(ctx, event, fmt.Sprintf("failed to authorize you in %s.", scm))
		return activities.AlertError(ctx, c.AlertsChannel, "authorize user", err,
			"User", fmt.Sprintf("<@%s>", event.UserID), "Thrippy ID", fmt.Sprintf("`%s`", thrippyID))
	}

	if err := data.UpsertUser(ctx, user.Email, user.RealName, "", "", user.SlackID, thrippyID); err != nil {
		commands.PostEphemeralError(ctx, event, "failed to write internal data about you.")
		err = errors.Join(err, c.deleteThrippyLink(ctx, thrippyID))
		return activities.AlertError(ctx, c.AlertsChannel, "upsert user", err,
			"User", fmt.Sprintf("<@%s>", event.UserID), "Thrippy ID", fmt.Sprintf("`%s`", thrippyID))
	}

	if err := commands.SetReminder(ctx, event, DefaultReminderTime, true); err != nil {
		err = errors.Join(err, c.deleteThrippyLink(ctx, thrippyID))
		return activities.AlertError(ctx, c.AlertsChannel, "set reminder", err, "User", fmt.Sprintf("<@%s>", event.UserID))
	}

	msg = ":bell: You are now opted into using RevChat.\n\n"
	msg += ":alarm_clock: Default time for weekday reminders = *8 AM* (in your current timezone). "
	msg += "To change it, run this Slack command:\n\n```%s reminders at <time in 12h or 24h format>```"
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, fmt.Sprintf(msg, event.Command))
}

func (c *Config) OptOutSlashCommand(ctx workflow.Context, event commands.SlashCommandEvent) error {
	user, optedIn, err := commands.UserDetails(ctx, event, event.UserID)
	if err != nil {
		return activities.AlertError(ctx, c.AlertsChannel, "user details", err, "User", fmt.Sprintf("<@%s>", event.UserID))
	}
	if !optedIn {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":no_bell: You're already opted out.")
	}

	data.DeleteReminder(ctx, user.SlackID)
	data.RemoveFollower(ctx, user.SlackID)

	if err := data.UpsertUser(ctx, user.Email, "", user.BitbucketID, user.GitHubID, user.SlackID, "X"); err != nil {
		commands.PostEphemeralError(ctx, event, "failed to write internal data about you.")
		return activities.AlertError(ctx, c.AlertsChannel, "upsert user", err, "User", fmt.Sprintf("<@%s>", event.UserID))
	}

	if err := c.deleteThrippyLink(ctx, user.ThrippyLink); err != nil {
		logger.From(ctx).Error("failed to delete Thrippy link for opted-out user",
			slog.Any("error", err), slog.String("thrippy_id", user.ThrippyLink))
		_ = activities.AlertError(ctx, c.AlertsChannel, "delete Thrippy link", err,
			"User", fmt.Sprintf("<@%s>", event.UserID), "Thrippy ID", fmt.Sprintf("`%s`", user.ThrippyLink))
		// This is an internal error, it doesn't concern or affect the user.
	}

	msg := ":no_bell: You are now opted out of using RevChat for new PRs."
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
