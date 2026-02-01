// Package temporal initializes a Temporal worker.
package temporal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/workflows"
	"github.com/tzrikka/revchat/pkg/config"
	github "github.com/tzrikka/revchat/pkg/github/workflows"
	slack "github.com/tzrikka/revchat/pkg/slack/workflows"
	"github.com/tzrikka/timpani-api/pkg/temporal"
)

// Run initializes the Temporal worker, and blocks to keep it running.
// This worker exposes (mostly asynchronous) Temporal workflows, and
// starts the event dispatcher workflow if it's not already running.
func Run(ctx context.Context, cmd *cli.Command, bi *debug.BuildInfo) error {
	l := logger.FromContext(ctx)
	addr := cmd.String("temporal-address")
	l.Info("Temporal server address: " + addr)

	copts := client.Options{
		HostPort:  addr,
		Namespace: cmd.String("temporal-namespace"),
		Logger:    log.NewStructuredLogger(l),
	}
	cli, err := client.Dial(copts)
	if err != nil {
		return fmt.Errorf("failed to dial Temporal: %w", err)
	}
	defer cli.Close()

	taskQueue := cmd.String("temporal-task-queue-revchat")
	w := worker.New(cli, taskQueue, worker.Options{
		DeploymentOptions: worker.DeploymentOptions{
			UseVersioning: true,
			Version: worker.WorkerDeploymentVersion{
				DeploymentName: "revchat",
				BuildID:        bi.Main.Version,
			},
			DefaultVersioningBehavior: workflow.VersioningBehaviorAutoUpgrade,
		},
	})
	bitbucket.RegisterPullRequestWorkflows(cmd, copts, taskQueue, w)
	bitbucket.RegisterRepositoryWorkflows(cmd, copts, taskQueue, w)
	github.RegisterWorkflows(cmd, w)
	slack.RegisterWorkflows(ctx, cmd, w)

	bitbucket.CreateSchedule(ctx, cli, taskQueue)
	slack.CreateSchedule(ctx, cli, taskQueue)

	temporal.ActivityOptions = &workflow.ActivityOptions{
		TaskQueue:           cmd.String("temporal-task-queue-timpani"),
		StartToCloseTimeout: config.StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: config.MaxRetryAttempts,
		},
	}

	cfg := new(Config)
	w.RegisterWorkflowWithOptions(cfg.EventDispatcherWorkflow, workflow.RegisterOptions{Name: EventDispatcher})
	run, err := cli.ExecuteWorkflow(ctx, workflowOptions(taskQueue), EventDispatcher)
	if err != nil {
		return fmt.Errorf("failed to start event dispatcher workflow: %w", err)
	}

	cfg.dispatcherWorkflowID = run.GetID()
	cfg.dispatcherRunID = run.GetRunID()

	if err := w.Run(cfg.interruptCh(ctx, cli)); err != nil {
		return fmt.Errorf("failed to start Temporal worker: %w", err)
	}

	return nil
}

// interruptCh starts a goroutine to trap SIGINT or SIGTERM signals from the OS. When such a
// signal is received, the goroutine sends a Temporal signal to the [EventDispatcherWorkflow] to
// start a graceful shutdown, and when that is done the workflow signals back to the Temporal worker
// using a native Go channel to comply with the OS signal in order to stop or restart the worker process.
// This function returns that native Go channel, which is passed to the worker's [worker.Worker.Run] call.
func (c *Config) interruptCh(ctx context.Context, cli client.Client) <-chan any {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-ch
		logger.FromContext(ctx).Info("received OS signal, shutting down gracefully", slog.String("signal", sig.String()))

		if err := cli.SignalWorkflow(ctx, c.dispatcherWorkflowID, c.dispatcherRunID, shutdownSignal, sig.String()); err != nil {
			logger.FatalContext(ctx, "failed to send shutdown signal to dispatcher workflow, exiting now", err)
		}
	}()

	done := make(chan any, 1)
	c.shutdownDone = done
	return done
}
