package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/config"
)

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
