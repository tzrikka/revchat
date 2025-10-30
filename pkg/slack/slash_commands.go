package slack

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// https://docs.slack.dev/interactivity/implementing-slash-commands/#app_command_handling
// https://docs.slack.dev/apis/events-api/using-socket-mode#command
type SlashCommandEvent struct {
	APIAppID string `json:"api_app_id"`

	IsEnterpriseInstall string `json:"is_enterprise_install"`
	EnterpriseID        string `json:"enterprise_id,omitempty"`
	EnterpriseName      string `json:"enterprise_name,omitempty"`
	TeamID              string `json:"team_id"`
	TeamDomain          string `json:"team_domain"`

	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`

	Command string `json:"command"`
	Text    string `json:"text"`

	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
}

const DefaultReminderTime = "8:00AM"

var (
	remindersPattern = regexp.MustCompile(`^reminders?(\s+at)?\s+(\S+)\s*(a|am|p|pm)?`)
	usersPattern     = regexp.MustCompile(`^(nudge|ping|set turn to)\s+`)
)

// https://docs.slack.dev/interactivity/implementing-slash-commands#app_command_handling
// https://docs.slack.dev/apis/events-api/using-socket-mode#command
func (c *Config) slashCommandWorkflow(ctx workflow.Context, event SlashCommandEvent) error {
	event.Text = strings.ToLower(event.Text)
	switch event.Text {
	case "", "help":
		return helpSlashCommand(ctx, event)
	case "opt-in", "opt in", "optin":
		return c.optInSlashCommand(ctx, event)
	case "opt-out", "opt out", "optout":
		return optOutSlashCommand(ctx, event)
	case "stat", "state", "status":
		return statusSlashCommand(ctx, event)
	case "my turn", "not my turn":
		return turnSlashCommand(ctx, event)
	case "approve", "lgtm", "+1":
		return approveSlashCommand(ctx, event)
	case "unapprove", "-1":
		return unapproveSlashCommand(ctx, event)
	}

	if remindersPattern.MatchString(event.Text) {
		return remindersSlashCommand(ctx, event)
	}
	if usersPattern.MatchString(event.Text) {
		return turnSlashCommand(ctx, event)
	}

	log.Warn(ctx, "unrecognized Slack slash command", "username", event.UserName, "text", event.Text)
	postEphemeralError(ctx, event, fmt.Sprintf("unrecognized command - try `%s help`", event.Command))
	return nil
}

func helpSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	msg := ":wave: Available slash commands for `%s`:\n\n"
	msg += "  •  `%s opt-in` - opt into being added to PR channels and receiving DMs\n"
	msg += "  •  `%s opt-out` - opt out of being added to PR channels and receiving DMs\n"
	msg += "  •  `%s reminders at &lt;time in 12h/24h format&gt;` (weekdays, using your timezone)\n"
	msg += "  •  `%s status` - show your current PR status, as an author and reviewer\n\n"
	msg += "More commands inside PR channels:\n\n"
	msg += "  •  `%s my turn` / `not my turn` / `set turn to &lt;1 or more @users&gt;`\n"
	msg += "  •  `%s nudge &lt;1 or more @users&gt;` or `ping &lt;1 or more @users&gt;`\n"
	msg += "  •  `%s approve` or `lgtm` or `+1`\n"
	msg += "  •  `%s unapprove` or `-1`\n"
	msg = strings.ReplaceAll(msg, "%s", event.Command)

	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func (c *Config) optInSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	email, err := users.SlackIDToEmail(ctx, event.UserID)
	if err != nil {
		return err
	}

	found, err := data.IsOptedIn(email)
	if err != nil {
		log.Error(ctx, "failed to load user opt-in status", "error", err, "email", email)
		postEphemeralError(ctx, event, "internal data reading error")
		return err
	}

	if found {
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":bell: You're already opted in.")
	}

	switch {
	case c.bitbucketWorkspace != "":
		return c.optInBitbucket(ctx, event, email)
	default:
		log.Error(ctx, "neither Bitbucket nor GitHub are configured")
		postEphemeralError(ctx, event, "internal configuration error")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c *Config) optInBitbucket(ctx workflow.Context, event SlashCommandEvent, email string) error {
	accountID, err := users.EmailToBitbucketID(ctx, c.bitbucketWorkspace, email)
	if err != nil {
		postEphemeralError(ctx, event, "internal data reading error")
		return err
	}

	linkID, nonce, err := c.createThrippyLink(ctx)
	if err != nil {
		log.Error(ctx, "failed to create Thrippy link for Slack user", "error", err)
		postEphemeralError(ctx, event, "internal authorization error")
		return err
	}

	msg := ":point_right: <https://%s/start?id=%s&nonce=%s|Click here> to authorize RevChat to act on your behalf in Bitbucket."
	msg = fmt.Sprintf(msg, c.thrippyHTTPAddress, linkID, nonce)
	if err := PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg); err != nil {
		log.Error(ctx, "failed to post ephemeral opt-in message in Slack", "error", err)
		return err
	}

	err = c.waitForThrippyLinkCreds(ctx, linkID)
	if err != nil {
		_ = c.deleteThrippyLink(ctx, linkID)
		if err.Error() == ErrLinkAuthzTimeout { // For some reason errors.Is() doesn't work across Temporal.
			log.Warn(ctx, "user did not complete Thrippy OAuth flow in time", "email", email)
			postEphemeralError(ctx, event, "Bitbucket authorization timed out - please try opting in again")
			return nil // Not a *server* error as far as we are concerned.
		} else {
			log.Error(ctx, "failed to authorize Bitbucket user", "error", err, "email", email)
			postEphemeralError(ctx, event, "failed to authorize you in Bitbucket")
			return err
		}
	}

	if err := data.OptInBitbucketUser(event.UserID, accountID, email, linkID); err != nil {
		log.Error(ctx, "failed to opt-in user", "error", err, "email", email)
		postEphemeralError(ctx, event, "internal data writing error")
		return err
	}

	if err := setReminder(ctx, event, DefaultReminderTime, true); err != nil {
		return err
	}

	msg = ":bell: You are now opted into using RevChat.\n\n"
	msg += ":alarm_clock: Default time for weekday reminders = **8 AM** (in your current timezone). "
	msg += "To change it, run this slash command:\n\n```/revchat reminders at &lt;time in 12h or 24h format&gt;```"
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func optOutSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	email, err := users.SlackIDToEmail(ctx, event.UserID)
	if err != nil {
		return err
	}

	found, err := data.IsOptedIn(email)
	if err != nil {
		log.Error(ctx, "failed to load user opt-in status", "error", err, "email", email)
		postEphemeralError(ctx, event, "internal data reading error")
		return err
	}

	if !found {
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":no_bell: You're already opted out.")
	}

	if err := data.OptOut(email); err != nil {
		log.Error(ctx, "failed to opt-out user", "error", err, "email", email)
		postEphemeralError(ctx, event, "internal data writing error")
		return err
	}

	msg := ":no_bell: You are now opted out of using RevChat for new PRs."
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func remindersSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	matches := remindersPattern.FindStringSubmatch(event.Text)
	if len(matches) < 3 {
		log.Error(ctx, "failed to parse reminders slash command - regex mismatch", "text", event.Text)
		postEphemeralError(ctx, event, "unexpected internal error while parsing command")
		return errors.New("failed to parse reminders command - regex mismatch")
	}

	amPm := ""
	if len(matches) > 3 {
		amPm = matches[3]
	}

	kitchenTime, err := normalizeTime(matches[2], amPm)
	if err != nil {
		log.Warn(ctx, "failed to parse time in reminders slash command", "error", err)
		postEphemeralError(ctx, event, err.Error())
		return nil // Not a *server* error as far as we are concerned.
	}

	if kt, _ := time.Parse(time.Kitchen, kitchenTime); kt.Minute() != 0 && kt.Minute() != 30 {
		log.Warn(ctx, "uncommon reminder time requested", "user_id", event.UserID, "time", kitchenTime)
		postEphemeralError(ctx, event, "please specify a time on the hour or half-hour")
		return nil // Not a *server* error as far as we are concerned.
	}

	return setReminder(ctx, event, kitchenTime, false)
}

func setReminder(ctx workflow.Context, event SlashCommandEvent, t string, quiet bool) error {
	user, err := slack.UsersInfoActivity(ctx, event.UserID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", event.UserID)
		postEphemeralError(ctx, event, "failed to retrieve Slack user info")
		return err
	}

	if user.TZ == "" {
		log.Warn(ctx, "Slack user has no timezone in their profile", "user_id", event.UserID)
		postEphemeralError(ctx, event, "please set a timezone in your Slack profile preferences first")
		return nil // Not a *server* error as far as we are concerned.
	}

	if _, err := time.LoadLocation(user.TZ); err != nil {
		log.Warn(ctx, "unrecognized user timezone", "error", err, "user_id", event.UserID, "tz", user.TZ)
		postEphemeralError(ctx, event, fmt.Sprintf("your Slack timezone is unrecognized: `%s`", user.TZ))
		return err
	}

	if err := data.SetReminder(event.UserID, t, user.TZ); err != nil {
		log.Error(ctx, "failed to store user reminder time", "error", err, "user_id", event.UserID, "time", t, "zone", user.TZ)
		postEphemeralError(ctx, event, "internal data writing error")
		return err
	}

	if !quiet {
		t = fmt.Sprintf("%s %s", t[:len(t)-2], t[len(t)-2:]) // Insert space before AM/PM suffix.
		msg := fmt.Sprintf(":alarm_clock: Your daily reminder time is set to **%s** _(%s)_ on weekdays.", t, user.TZ)
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	return nil
}

func statusSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	log.Warn(ctx, "status slash command not implemented yet")
	postEphemeralError(ctx, event, "not implemented yet")
	return nil
}

func turnSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	log.Warn(ctx, "turn slash command not implemented yet")
	postEphemeralError(ctx, event, "not implemented yet")
	return nil
}

func approveSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	log.Warn(ctx, "approve slash command not implemented yet")
	postEphemeralError(ctx, event, "not implemented yet")
	return nil
}

func unapproveSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	log.Warn(ctx, "unapprove slash command not implemented yet")
	postEphemeralError(ctx, event, "not implemented yet")
	return nil
}

func postEphemeralError(ctx workflow.Context, event SlashCommandEvent, msg string) {
	msg = fmt.Sprintf(":warning: Error in `%s %s`: %s", event.Command, event.Text, msg)
	// We're already reporting another error, there's nothing to do if this fails.
	_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
