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
		return helpSlashCommand(ctx, event)
	case "opt-in", "opt in", "optin":
		return c.optInSlashCommand(ctx, event)
	case "opt-out", "opt out", "optout":
		return c.optOutSlashCommand(ctx, event)
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

	return c.optInBitbucket(ctx, event, email)
}

func (c Config) optInBitbucket(ctx workflow.Context, event SlashCommandEvent, email string) error {
	accountID, err := users.EmailToBitbucketID(ctx, "workspace", email)
	if err != nil {
		postEphemeralError(ctx, event, "internal data reading error")
		return err
	}

	if err := data.OptInBitbucketUser(event.UserID, accountID, email); err != nil {
		log.Error(ctx, "failed to opt-in user", "error", err, "email", email)
		postEphemeralError(ctx, event, "internal data writing error")
		return err
	}

	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":bell: You are now opted into using RevChat")
}

func (c Config) optOutSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
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
