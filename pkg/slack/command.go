package slack

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.slack.dev/interactivity/implementing-slash-commands#app_command_handling
// https://docs.slack.dev/apis/events-api/using-socket-mode#command
func (c Config) slashCommandWorkflow(ctx workflow.Context, event map[string]any) error {
	log.Warn(ctx, "slash command event", "event", event)
	return nil
}
