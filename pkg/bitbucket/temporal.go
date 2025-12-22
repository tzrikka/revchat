package bitbucket

import (
	"log/slog"
	"strings"

	"github.com/urfave/cli/v3"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/metrics"
)

type Config struct {
	SlackChannelNamePrefix    string
	SlackChannelNameMaxLength int
	SlackChannelsArePrivate   bool

	LinkifyMap map[string]string
}

func newConfig(cmd *cli.Command) *Config {
	return &Config{
		SlackChannelNamePrefix:    cmd.String("slack-channel-name-prefix"),
		SlackChannelNameMaxLength: cmd.Int("slack-channel-name-max-length"),
		SlackChannelsArePrivate:   cmd.Bool("slack-private-channels"),

		LinkifyMap: config.KVSliceToMap(cmd.StringSlice("linkification-map")),
	}
}

type (
	prWorkflowFunc   func(workflow.Context, PullRequestEvent) error
	repoWorkflowFunc func(workflow.Context, RepositoryEvent) error
)

// PullRequestSignals is a list of signal names that RevChat
// receives from Timpani, to trigger event handling workflows.
//
// This is based on:
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Pull-request-events
//   - https://github.com/tzrikka/revchat/blob/main/docs/setup/bitbucket.md#webhook-triggers
//   - https://github.com/tzrikka/timpani/blob/main/pkg/listeners/bitbucket/webhook.go
var PullRequestSignals = []string{
	"bitbucket.events.pullrequest.created",
	"bitbucket.events.pullrequest.updated",
	"bitbucket.events.pullrequest.approved",
	"bitbucket.events.pullrequest.unapproved",
	"bitbucket.events.pullrequest.changes_request_created",
	"bitbucket.events.pullrequest.changes_request_removed",
	"bitbucket.events.pullrequest.fulfilled", // Merged.
	"bitbucket.events.pullrequest.rejected",  // Declined.

	"bitbucket.events.pullrequest.comment_created",
	"bitbucket.events.pullrequest.comment_updated",
	"bitbucket.events.pullrequest.comment_deleted",
	"bitbucket.events.pullrequest.comment_resolved",
	"bitbucket.events.pullrequest.comment_reopened",
}

// RepositorySignals is a list of signal names that RevChat
// receives from Timpani, to trigger event handling workflows.
//
// This is based on:
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Repository-events
//   - https://github.com/tzrikka/revchat/blob/main/docs/setup/bitbucket.md#webhook-triggers
//   - https://github.com/tzrikka/timpani/blob/main/pkg/listeners/bitbucket/webhook.go
var RepositorySignals = []string{
	"bitbucket.events.repo.commit_comment_created",

	"bitbucket.events.repo.commit_status_created",
	"bitbucket.events.repo.commit_status_updated",
}

// RegisterPullRequestWorkflows maps event-handling workflow functions to [PullRequestSignals].
func RegisterPullRequestWorkflows(cmd *cli.Command, w worker.Worker) {
	c := newConfig(cmd)
	funcs := []prWorkflowFunc{
		c.prCreatedWorkflow,
		c.prUpdatedWorkflow,
		prReviewedWorkflow, // Approved.
		prReviewedWorkflow, // Unapproved.
		prReviewedWorkflow, // Changes requested.
		prReviewedWorkflow, // Changes request removed.
		prClosedWorkflow,   // Fulfilled, a.k.a. merged.
		prClosedWorkflow,   // Rejected, a.k.a. declined.

		prCommentCreatedWorkflow,
		prCommentUpdatedWorkflow,
		prCommentDeletedWorkflow,
		prCommentResolvedWorkflow,
		prCommentReopenedWorkflow,
	}
	for i, f := range funcs {
		w.RegisterWorkflowWithOptions(f, workflow.RegisterOptions{Name: PullRequestSignals[i]})
	}
}

// RegisterRepositoryWorkflows maps event-handling workflow functions to [RepositorySignals].
func RegisterRepositoryWorkflows(w worker.Worker) {
	funcs := []repoWorkflowFunc{
		commitCommentCreatedWorkflow,

		commitStatusWorkflow,
		commitStatusWorkflow,
	}
	for i, f := range funcs {
		w.RegisterWorkflowWithOptions(f, workflow.RegisterOptions{Name: RepositorySignals[i]})
	}
}

// RegisterPullRequestSignals routes [PullRequestSignals] to their registered workflows.
func RegisterPullRequestSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	for _, signalName := range PullRequestSignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
			payload := new(PullRequestEvent)
			ch.Receive(ctx, payload)

			signal := ch.Name()
			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.pullrequest.")
			logger.Info(ctx, "received signal", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)

			// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
			opts := workflow.ChildWorkflowOptions{TaskQueue: taskQueue}
			if payload.Type != "created" {
				opts.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
			}
			ctx = workflow.WithChildOptions(ctx, opts)
			wf := workflow.ExecuteChildWorkflow(ctx, signal, payload)

			// Wait for [Config.prCreated] completion before returning, to ensure we handle
			// subsequent PR initialization events appropriately (e.g. check states).
			if payload.Type == "created" {
				_ = wf.Get(ctx, nil) // Blocks until child workflow completes.
			} else {
				_ = wf.GetChildWorkflowExecution().Get(ctx, nil) // Blocks until child workflow starts.
			}
		})
	}
}

// RegisterRepositorySignals routes [RepositorySignals] to their registered workflows.
func RegisterRepositorySignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	for _, signalName := range RepositorySignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
			payload := new(RepositoryEvent)
			ch.Receive(ctx, payload)

			signal := ch.Name()
			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.repo.")
			logger.Info(ctx, "received signal", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)

			// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
				TaskQueue:         taskQueue,
			})
			_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).GetChildWorkflowExecution().Get(ctx, nil)
		})
	}
}

// DrainPullRequestSignals drains all pending [PullRequestSignals] channels,
// and waits for their corresponding workflow executions to complete in order.
// This is called in preparation for resetting the dispatcher workflow's history.
func DrainPullRequestSignals(ctx workflow.Context, taskQueue string) bool {
	totalEvents := 0
	for _, signal := range PullRequestSignals {
		ch := workflow.GetSignalChannel(ctx, signal)
		signalEvents := 0
		for {
			payload := new(PullRequestEvent)
			if !ch.ReceiveAsync(payload) {
				break
			}

			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.pullrequest.")
			logger.Info(ctx, "received signal while draining", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)
			signalEvents++

			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
			_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).Get(ctx, nil)
		}

		if signalEvents > 0 {
			logger.Info(ctx, "drained signal channel", slog.String("signal", signal), slog.Int("event_count", signalEvents))
		}
		totalEvents += signalEvents
	}
	return totalEvents > 0
}

// DrainRepositorySignals drains all pending [RepositorySignals] channels,
// and waits for their corresponding workflow executions to complete in order.
// This is called in preparation for resetting the dispatcher workflow's history.
func DrainRepositorySignals(ctx workflow.Context, taskQueue string) bool {
	totalEvents := 0
	for _, signal := range RepositorySignals {
		ch := workflow.GetSignalChannel(ctx, signal)
		signalEvents := 0
		for {
			payload := new(RepositoryEvent)
			if !ch.ReceiveAsync(payload) {
				break
			}

			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.repo.")
			logger.Info(ctx, "received signal while draining", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)
			signalEvents++

			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
			_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).Get(ctx, nil)
		}

		if signalEvents > 0 {
			logger.Info(ctx, "drained signal channel", slog.String("signal", signal), slog.Int("event_count", signalEvents))
		}
		totalEvents += signalEvents
	}
	return totalEvents > 0
}
