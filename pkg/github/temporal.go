package github

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/config"
)

type GitHub struct {
	cmd *cli.Command
}

func RegisterWorkflows(w worker.Worker, cmd *cli.Command) {
	g := GitHub{cmd: cmd}
	w.RegisterWorkflowWithOptions(g.eventsWorkflow, workflow.RegisterOptions{Name: "github.events"})
	w.RegisterWorkflowWithOptions(g.archiveChannelWorkflow, workflow.RegisterOptions{Name: "github.archiveChannel"})
	w.RegisterWorkflowWithOptions(g.initChannelWorkflow, workflow.RegisterOptions{Name: "github.initChannel"})
}

func (g GitHub) executeRevChatWorkflow(ctx workflow.Context, name string, req any) workflow.ChildWorkflowFuture {
	tq := g.cmd.String("temporal-task-queue-revchat")
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: tq})
	return workflow.ExecuteChildWorkflow(ctx, name, req)
}

// executeTimpaniActivity requests the execution of a [Timpani] activity in the context of
// a Temporal workflow, with preconfigured activity options related to timeouts and retries.
//
// [Timpani]: https://github.com/tzrikka/timpani/tree/main/pkg/api/github
func (g GitHub) executeTimpaniActivity(ctx workflow.Context, name string, req any) workflow.Future {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:              g.cmd.String("temporal-task-queue-timpani"),
		ScheduleToStartTimeout: config.ScheduleToStartTimeout,
		StartToCloseTimeout:    config.StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: config.MaxRetryAttempts,
		},
	})

	return workflow.ExecuteActivity(ctx, name, req)
}
