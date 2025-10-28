// Package temporal initializes a Temporal worker.
package temporal

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/github"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/timpani-api/pkg/temporal"
)

// Run initializes the Temporal worker, and blocks to keep it running.
// This worker exposes (mostly asynchronous) Temporal workflows, and
// starts the event dispatcher workflow if it's not already running.
func Run(ctx context.Context, l zerolog.Logger, cmd *cli.Command) error {
	addr := cmd.String("temporal-address")
	l.Info().Msgf("Temporal server address: %s", addr)

	c, err := client.Dial(client.Options{
		HostPort:  addr,
		Namespace: cmd.String("temporal-namespace"),
		Logger:    LogAdapter{zerolog: l.With().CallerWithSkipFrameCount(8).Logger()},
	})
	if err != nil {
		return fmt.Errorf("failed to dial Temporal: %w", err)
	}
	defer c.Close()

	ctx = l.WithContext(ctx)
	w := worker.New(c, cmd.String("temporal-task-queue-revchat"), worker.Options{})
	bitbucket.RegisterPullRequestWorkflows(w, cmd)
	bitbucket.RegisterRepositoryWorkflows(w, cmd)
	github.RegisterWorkflows(w, cmd)
	slack.RegisterWorkflows(ctx, w, cmd)

	slack.CreateSchedule(ctx, c, cmd)

	cfg := Config{cmd: cmd}
	w.RegisterWorkflowWithOptions(cfg.EventDispatcherWorkflow, workflow.RegisterOptions{Name: EventDispatcher})

	opts := client.StartWorkflowOptions{
		ID:                       EventDispatcher,
		TaskQueue:                cmd.String("temporal-task-queue-revchat"),
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}
	if _, err := c.ExecuteWorkflow(ctx, opts, EventDispatcher); err != nil {
		return fmt.Errorf("failed to start event dispatcher workflow: %w", err)
	}

	temporal.ActivityOptions = &workflow.ActivityOptions{
		TaskQueue:           cmd.String("temporal-task-queue-timpani"),
		StartToCloseTimeout: config.StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: config.MaxRetryAttempts,
		},
	}

	if err := w.Run(worker.InterruptCh()); err != nil {
		return fmt.Errorf("failed to start Temporal worker: %w", err)
	}

	return nil
}
