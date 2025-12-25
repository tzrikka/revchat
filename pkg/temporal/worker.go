// Package temporal initializes a Temporal worker.
package temporal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/workflows"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/github"
	slack "github.com/tzrikka/revchat/pkg/slack/workflows"
	"github.com/tzrikka/timpani-api/pkg/temporal"
)

// Run initializes the Temporal worker, and blocks to keep it running.
// This worker exposes (mostly asynchronous) Temporal workflows, and
// starts the event dispatcher workflow if it's not already running.
func Run(ctx context.Context, cmd *cli.Command) error {
	l := logger.FromContext(ctx)
	addr := cmd.String("temporal-address")
	l.Info("Temporal server address: " + addr)

	c, err := client.Dial(client.Options{
		HostPort:  addr,
		Namespace: cmd.String("temporal-namespace"),
		Logger:    log.NewStructuredLogger(l),
	})
	if err != nil {
		return fmt.Errorf("failed to dial Temporal: %w", err)
	}
	defer c.Close()

	tq := cmd.String("temporal-task-queue-revchat")
	w := worker.New(c, tq, worker.Options{})
	bitbucket.RegisterPullRequestWorkflows(cmd, w)
	bitbucket.RegisterRepositoryWorkflows(w)
	github.RegisterWorkflows(w, cmd)
	slack.RegisterWorkflows(ctx, w, cmd)

	slack.CreateSchedule(ctx, c, cmd)

	cfg := &Config{taskQueue: tq}
	opts := client.StartWorkflowOptions{
		ID:                       EventDispatcher,
		TaskQueue:                tq,
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}
	temporal.ActivityOptions = &workflow.ActivityOptions{
		TaskQueue:           cmd.String("temporal-task-queue-timpani"),
		StartToCloseTimeout: config.StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: config.MaxRetryAttempts,
		},
	}

	w.RegisterWorkflowWithOptions(cfg.EventDispatcherWorkflow, workflow.RegisterOptions{Name: EventDispatcher})
	workflowRun, err := c.ExecuteWorkflow(ctx, opts, EventDispatcher)
	if err != nil {
		return fmt.Errorf("failed to start event dispatcher workflow: %w", err)
	}

	if err := w.Run(interruptCh(ctx, c, workflowRun, cfg)); err != nil {
		return fmt.Errorf("failed to start Temporal worker: %w", err)
	}

	return nil
}

// interruptCh starts a goroutine to trap SIGINT or SIGTERM signals from the OS. When such a
// signal is received, the goroutine sends a Temporal signal to the [EventDispatcherWorkflow] to
// start a graceful shutdown, and when that is done the workflow signals back to the Temporal worker
// using a native Go channel to comply with the OS signal in order to stop or restart the worker process.
// This function returns that native Go channel, which is passed to the worker's [worker.Worker.Run] call.
func interruptCh(ctx context.Context, c client.Client, run client.WorkflowRun, cfg *Config) <-chan any {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-ch
		signal.Stop(ch)
		logger.FromContext(ctx).Info("received OS signal, shutting down gracefully", slog.String("signal", sig.String()))
		if err := c.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, sig.String()); err != nil {
			logger.FatalContext(ctx, "failed to send shutdown signal to dispatcher workflow, exiting now", err)
		}
	}()

	done := make(chan any, 1)
	cfg.shutdownDone = done
	return done
}
