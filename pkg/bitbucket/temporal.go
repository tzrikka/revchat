package bitbucket

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
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

func (b Bitbucket) executeWorkflow(ctx workflow.Context, name string, req any) workflow.ChildWorkflowFuture {
	tq := b.cmd.String("temporal-task-queue-revchat")
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: tq})
	return workflow.ExecuteChildWorkflow(ctx, name, req)
}
