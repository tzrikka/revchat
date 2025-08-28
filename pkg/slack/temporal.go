package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/config"
)

type Config struct {
	Cmd *cli.Command
}

// Signals is a list of signal names that RevChat receives
// from Timpani, to trigger event handling workflows.
//
// This is based on:
//   - https://github.com/tzrikka/revchat/blob/main/docs/slack.md#bot-event-subscriptions
//   - https://github.com/tzrikka/timpani/blob/main/pkg/listeners/slack/dispatch.go
var Signals = []string{
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
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[4]})
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[5]})
}

// RegisterSignals routes [Signals] to registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})

	for _, sig := range Signals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, sig), func(c workflow.ReceiveChannel, _ bool) {
			e := map[string]any{}
			c.Receive(ctx, &e)

			log.Info(ctx, "received signal", "signal", c.Name())
			workflow.ExecuteChildWorkflow(childCtx, c.Name(), e)
		})
	}
}

// executeTimpaniActivity requests the execution of a [Timpani] activity in the context of
// a Temporal workflow, with preconfigured activity options related to timeouts and retries.
//
// [Timpani]: https://github.com/tzrikka/timpani/tree/main/pkg/api/slack
func executeTimpaniActivity(ctx workflow.Context, cmd *cli.Command, name string, req any) workflow.Future {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:              cmd.String("temporal-task-queue-timpani"),
		ScheduleToStartTimeout: config.ScheduleToStartTimeout,
		StartToCloseTimeout:    config.StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: config.MaxRetryAttempts,
		},
	})

	return workflow.ExecuteActivity(ctx, name, req)
}
