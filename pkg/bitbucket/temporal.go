package bitbucket

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/config"
)

type Bitbucket struct {
	cmd *cli.Command
}

func RegisterWorkflows(w worker.Worker, cmd *cli.Command) {
	b := Bitbucket{cmd: cmd}
	w.RegisterWorkflowWithOptions(b.eventsWorkflow, workflow.RegisterOptions{Name: "bitbucket.events"})
	w.RegisterWorkflowWithOptions(b.archiveChannelWorkflow, workflow.RegisterOptions{Name: "bitbucket.archiveChannel"})
	w.RegisterWorkflowWithOptions(b.initChannelWorkflow, workflow.RegisterOptions{Name: "bitbucket.initChannel"})
}

func (b Bitbucket) executeRevChatWorkflow(ctx workflow.Context, name string, req any) workflow.ChildWorkflowFuture {
	tq := b.cmd.String("temporal-task-queue-revchat")
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: tq})
	return workflow.ExecuteChildWorkflow(ctx, name, req)
}

// executeTimpaniActivity requests the execution of a [Timpani] activity in the context of
// a Temporal workflow, with preconfigured activity options related to timeouts and retries.
//
// [Timpani]: https://github.com/tzrikka/timpani/tree/main/pkg/api/bitbucket
func (b Bitbucket) executeTimpaniActivity(ctx workflow.Context, name string, req any) workflow.Future {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:              b.cmd.String("temporal-task-queue-timpani"),
		ScheduleToStartTimeout: config.ScheduleToStartTimeout,
		StartToCloseTimeout:    config.StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: config.MaxRetryAttempts,
		},
	})

	return workflow.ExecuteActivity(ctx, name, req)
}
