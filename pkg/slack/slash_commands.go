package slack

import (
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
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

// https://docs.slack.dev/interactivity/implementing-slash-commands#app_command_handling
// https://docs.slack.dev/apis/events-api/using-socket-mode#command
func (c Config) slashCommandWorkflow(ctx workflow.Context, event SlashCommandEvent) error {
	event.Text = strings.ToLower(event.Text)
	switch event.Text {
	case "", "help":
		return helpSlashCommand(ctx, event)
	case "opt-in", "opt in", "optin":
		return c.optInSlashCommand(ctx, event)
	case "opt-out", "opt out", "optout":
		return optOutSlashCommand(ctx, event)
	}

	log.Warn(ctx, "unrecognized Slack command", "username", event.UserName, "text", event.Text)
	return nil
}

func helpSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	msg := ":wave: Available slash commands for `%s`:\n\n"
	msg += "  •  `%s opt-in` - opt into being added to PR channels and receiving DMs\n"
	msg += "  •  `%s opt-out` - opt out of being added to PR channels and receiving DMs\n"
	msg = strings.ReplaceAll(msg, "%s", event.Command)

	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func (c Config) optInSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
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
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":bell: You're already opted in")
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

func (c Config) optInBitbucket(ctx workflow.Context, event SlashCommandEvent, email string) error {
	accountID, err := users.EmailToBitbucketID(ctx, c.bitbucketWorkspace, email)
	if err != nil {
		postEphemeralError(ctx, event, "internal data reading error")
		return err
	}

	linkID, err := c.createThrippyLink(ctx)
	if err != nil {
		log.Error(ctx, "failed to create Thrippy link for Slack user", "error", err)
		postEphemeralError(ctx, event, "internal authorization error")
		return err
	}

	msg := ":point_right: <https://%s/start?id=%s|Click here> to authorize RevChat to act on your behalf in Bitbucket"
	msg = fmt.Sprintf(msg, c.thrippyHTTPAddress, linkID)
	if err := PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg); err != nil {
		log.Error(ctx, "failed to post ephemeral opt-in message in Slack", "error", err)
		return err
	}

	err = c.waitForThrippyLinkCreds(ctx, linkID)
	if err != nil {
		c.deleteThrippyLink(ctx, linkID)
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

	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":bell: You are now opted into using RevChat")
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
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":no_bell: You're already opted out")
	}

	if err := data.OptOut(email); err != nil {
		log.Error(ctx, "failed to opt-out user", "error", err, "email", email)
		postEphemeralError(ctx, event, "internal data writing error")
		return err
	}

	msg := ":no_bell: You are now opted out of using RevChat for new PRs"
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func postEphemeralError(ctx workflow.Context, event SlashCommandEvent, msg string) {
	msg = fmt.Sprintf(":warning: Error in `%s %s`: %s", event.Command, event.Text, msg)
	// We're already reporting another error, there's nothing to do if this fails.
	_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
