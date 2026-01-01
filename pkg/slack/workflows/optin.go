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

func (c *Config) OptInSlashCommand(ctx workflow.Context, event commands.SlashCommandEvent) error {
	user, optedIn, err := commands.UserDetails(ctx, event, event.UserID)
	if err != nil {
		return err
	}
	if optedIn {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":bell: You're already opted in.")
	}

	info, err := slack.UsersInfo(ctx, event.UserID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", event.UserID))
		commands.PostEphemeralError(ctx, event, "failed to retrieve your user info from Slack.")
		return err
	}

	// Ensure the user's basic details are set even if they're unrecognized.
	user.RealName = info.RealName
	user.SlackID = event.UserID
	user.Email = strings.ToLower(info.Profile.Email)
	if info.IsBot {
		user.Email = "bot"
	}

	switch {
	case c.BitbucketWorkspace != "":
		return c.optInBitbucket(ctx, event, user)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		commands.PostEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c *Config) optInBitbucket(ctx workflow.Context, event commands.SlashCommandEvent, user data.User) error {
	linkID, nonce, err := c.createThrippyLink(ctx)
	if err != nil {
		logger.From(ctx).Error("failed to create Thrippy link for Slack user", slog.Any("error", err))
		commands.PostEphemeralError(ctx, event, "internal authorization failure.")
		return err
	}

	msg := ":point_right: <https://%s/start?id=%s&nonce=%s|Click here> to authorize RevChat to act on your behalf in Bitbucket."
	msg = fmt.Sprintf(msg, c.ThrippyHTTPAddress, linkID, nonce)
	if err := activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg); err != nil {
		logger.From(ctx).Error("failed to post ephemeral opt-in message in Slack", slog.Any("error", err))
		_ = c.deleteThrippyLink(ctx, linkID)
		return err
	}

	err = c.waitForThrippyLinkCreds(ctx, linkID)
	if err != nil {
		_ = c.deleteThrippyLink(ctx, linkID)
		if err.Error() == errLinkAuthzTimeout { // For some reason errors.Is() doesn't work across Temporal?
			logger.From(ctx).Warn("user did not complete Thrippy OAuth flow in time", slog.String("email", user.Email))
			commands.PostEphemeralError(ctx, event, "Bitbucket authorization timed out - please try opting in again.")
			return nil // Not a server error as far as we're concerned.
		} else {
			logger.From(ctx).Error("failed to authorize Bitbucket user",
				slog.Any("error", err), slog.String("email", user.Email))
			commands.PostEphemeralError(ctx, event, "failed to authorize you in Bitbucket.")
			return err
		}
	}

	if err := data.UpsertUser(ctx, user.Email, user.RealName, "", "", user.SlackID, linkID); err != nil {
		commands.PostEphemeralError(ctx, event, "failed to write internal data about you.")
		_ = c.deleteThrippyLink(ctx, linkID)
		return err
	}

	if err := commands.SetReminder(ctx, event, DefaultReminderTime, true); err != nil {
		_ = c.deleteThrippyLink(ctx, linkID)
		return err
	}

	msg = ":bell: You are now opted into using RevChat.\n\n"
	msg += ":alarm_clock: Default time for weekday reminders = *8 AM* (in your current timezone). "
	msg += "To change it, run this slash command:\n\n```%s reminders at <time in 12h or 24h format>```"
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, fmt.Sprintf(msg, event.Command))
}

func (c *Config) OptOutSlashCommand(ctx workflow.Context, event commands.SlashCommandEvent) error {
	user, optedIn, err := commands.UserDetails(ctx, event, event.UserID)
	if err != nil {
		return err
	}
	if !optedIn {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":no_bell: You're already opted out.")
	}

	if err := data.UpsertUser(ctx, user.Email, user.RealName, user.BitbucketID, user.GitHubID, user.SlackID, "X"); err != nil {
		commands.PostEphemeralError(ctx, event, "failed to write internal data about you.")
		return err
	}

	if err := c.deleteThrippyLink(ctx, user.ThrippyLink); err != nil {
		logger.From(ctx).Error("failed to delete Thrippy link for opted-out user",
			slog.Any("error", err), slog.String("link_id", user.ThrippyLink))
		// This is an internal error, it doesn't concern or affect the user.
	}

	msg := ":no_bell: You are now opted out of using RevChat for new PRs."
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
