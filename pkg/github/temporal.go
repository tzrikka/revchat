package github

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type GitHub struct {
	cmd *cli.Command
}

func RegisterWorkflows(w worker.Worker, cmd *cli.Command) {
	g := GitHub{cmd: cmd}
	w.RegisterWorkflowWithOptions(g.eventsWorkflow, workflow.RegisterOptions{Name: "github.events"})
	w.RegisterWorkflowWithOptions(g.archiveChannelWorkflow, workflow.RegisterOptions{Name: "github.archiveChannel"})
	w.RegisterWorkflowWithOptions(g.initChannelWorkflow, workflow.RegisterOptions{Name: "github.initChannel"})
	w.RegisterWorkflowWithOptions(g.updateMembersWorkflow, workflow.RegisterOptions{Name: "github.updateMembers"})
}

func (g GitHub) executeWorkflow(ctx workflow.Context, name string, req any) workflow.ChildWorkflowFuture {
	tq := g.cmd.String("temporal-task-queue-revchat")
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: tq})
	return workflow.ExecuteChildWorkflow(ctx, name, req)
}
