package slack

import (
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
		return c.helpSlashCommand(ctx, event)
	case "opt-in", "opt in", "optin":
		return c.optInSlashCommand(ctx, event)
	case "opt-out", "opt out", "optout":
		return c.optOutSlashCommand(ctx, event)
	}

	log.Warn(ctx, "unrecognized Slack command", "username", event.UserName, "text", event.Text)
	return nil
}

func (c Config) helpSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	msg := ":wave: Available slash commands for `%s`:\n\n"
	msg += "  •  `%s opt-in` - opt into being added to PR channels and receiving DMs\n"
	msg += "  •  `%s opt-out` - opt out of being added to PR channels and receiving DMs\n"
	msg = strings.ReplaceAll(msg, "%s", event.Command)

	return PostEphemeralMessageActivity(ctx, c.Cmd, event.ChannelID, event.UserID, msg)
}

func (c Config) optInSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	email, err := users.SlackIDToEmail(ctx, c.Cmd, event.UserID)
	if err != nil {
		return err
	}

	found, err := data.IsOptedIn(email)
	if err != nil {
		log.Error(ctx, "failed to load user opt-in status", "error", err, "email", email)
		c.postEphemeralError(ctx, event, "internal data reading error")
		return err
	}

	if found {
		msg := ":bell: You're already opted in"
		return PostEphemeralMessageActivity(ctx, c.Cmd, event.ChannelID, event.UserID, msg)
	}

	return c.optInBitbucket(ctx, event, email)
}

func (c Config) optInBitbucket(ctx workflow.Context, event SlashCommandEvent, email string) error {
	accountID := "TODO"

	if err := data.OptInBitbucketUser(event.UserID, accountID, email); err != nil {
		log.Error(ctx, "failed to opt-in user", "error", err, "email", email)
		c.postEphemeralError(ctx, event, "internal data writing error")
		return err
	}

	msg := ":bell: You are now opted into using RevChat"
	return PostEphemeralMessageActivity(ctx, c.Cmd, event.ChannelID, event.UserID, msg)
}

func (c Config) optOutSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	email, err := users.SlackIDToEmail(ctx, c.Cmd, event.UserID)
	if err != nil {
		return err
	}

	found, err := data.IsOptedIn(email)
	if err != nil {
		log.Error(ctx, "failed to load user opt-in status", "error", err, "email", email)
		c.postEphemeralError(ctx, event, "internal data reading error")
		return err
	}

	if !found {
		msg := ":no_bell: You're already opted out"
		return PostEphemeralMessageActivity(ctx, c.Cmd, event.ChannelID, event.UserID, msg)
	}

	if err := data.OptOut(email); err != nil {
		log.Error(ctx, "failed to opt-out user", "error", err, "email", email)
		c.postEphemeralError(ctx, event, "internal data writing error")
		return err
	}

	msg := ":no_bell: You are now opted out of using RevChat for new PRs"
	return PostEphemeralMessageActivity(ctx, c.Cmd, event.ChannelID, event.UserID, msg)
}

func (c Config) postEphemeralError(ctx workflow.Context, event SlashCommandEvent, msg string) {
	msg = fmt.Sprintf(":warning: Error in `%s %s`: %s", event.Command, event.Text, msg)
	_ = PostEphemeralMessageActivity(ctx, c.Cmd, event.ChannelID, event.UserID, msg)
}
