package slack

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
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
	log.Warn(ctx, "slash command event", "event", event)
	return nil
}
