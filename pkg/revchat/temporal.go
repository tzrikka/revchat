package revchat

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

func RegisterGitHubWorkflows(w worker.Worker, cmd *cli.Command) {
	g := GitHub{cmd: cmd}
	w.RegisterWorkflowWithOptions(g.EventsWorkflow, workflow.RegisterOptions{Name: "github.events"})
	w.RegisterWorkflowWithOptions(g.InitChannelWorkflow, workflow.RegisterOptions{Name: "github.initChannel"})
}

func (g GitHub) executeRevChatWorkflow(ctx workflow.Context, name string, req any) workflow.ChildWorkflowFuture {
	tq := g.cmd.String("temporal-task-queue-revchat")
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: tq})
	return workflow.ExecuteChildWorkflow(ctx, name, req)
}

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

type Slack struct {
	cmd *cli.Command
}

func RegisterSlackWorkflows(w worker.Worker, cmd *cli.Command) {
	s := Slack{cmd: cmd}
	w.RegisterWorkflowWithOptions(s.ArchiveChannelWorkflow, workflow.RegisterOptions{Name: "slack.archiveChannel"})
	w.RegisterWorkflowWithOptions(s.CreateChannelWorkflow, workflow.RegisterOptions{Name: "slack.createChannel"})
}

func (s Slack) executeTimpaniActivity(ctx workflow.Context, name string, req any) workflow.Future {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:              s.cmd.String("temporal-task-queue-timpani"),
		ScheduleToStartTimeout: config.ScheduleToStartTimeout,
		StartToCloseTimeout:    config.StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: config.MaxRetryAttempts,
		},
	})

	return workflow.ExecuteActivity(ctx, name, req)
}
