package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

type Config struct {
	Cmd *cli.Command
}

// Signals is a list of signal names that RevChat receives
// from Timpani, to trigger event handling workflows.
//
// This is based on:
//   - https://docs.slack.dev/reference/events?APIs=Events
//   - https://github.com/tzrikka/revchat/blob/main/docs/slack.md#bot-event-subscriptions
//   - https://github.com/tzrikka/timpani/blob/main/pkg/listeners/slack/dispatch.go
var Signals = []string{
	"slack.events.app_rate_limited",

	"slack.events.member_joined_channel",
	"slack.events.member_left_channel",
	"slack.events.message",
	"slack.events.reaction_added",
	"slack.events.reaction_removed",

	"slack.events.slash_command",
}

// RegisterWorkflows maps event handler functions to [Signals].
func RegisterWorkflows(w worker.Worker, cmd *cli.Command) {
	c := Config{Cmd: cmd}
	w.RegisterWorkflowWithOptions(c.appRateLimitedWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(c.memberJoinedWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(c.memberLeftWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(c.messageWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(c.reactionAddedWorkflow, workflow.RegisterOptions{Name: Signals[4]})
	w.RegisterWorkflowWithOptions(c.reactionRemovedWorkflow, workflow.RegisterOptions{Name: Signals[5]})
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[6]})
}

// RegisterSignals routes [Signals] to registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})

	addSignalHandler[map[string]any](sel, ctx, childCtx, Signals[0])
	addSignalHandler[memberEventWrapper](sel, ctx, childCtx, Signals[1])
	addSignalHandler[memberEventWrapper](sel, ctx, childCtx, Signals[2])
	addSignalHandler[messageEventWrapper](sel, ctx, childCtx, Signals[3])
	addSignalHandler[reactionEventWrapper](sel, ctx, childCtx, Signals[4])
	addSignalHandler[reactionEventWrapper](sel, ctx, childCtx, Signals[5])
	addSignalHandler[SlashCommandEvent](sel, ctx, childCtx, Signals[6])
}

func addSignalHandler[T any](sel workflow.Selector, ctx, childCtx workflow.Context, signal string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, signal), func(c workflow.ReceiveChannel, _ bool) {
		var event T
		c.Receive(ctx, &event)

		log.Info(ctx, "received signal", "signal", c.Name())
		workflow.ExecuteChildWorkflow(childCtx, c.Name(), event)
	})
}
