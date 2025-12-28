package workflows

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/metrics"
)

type Config struct {
	SlackChannelNamePrefix    string
	SlackChannelNameMaxLength int
	SlackChannelsArePrivate   bool

	LinkifyMap map[string]string

	Opts client.Options
	TQ   string
}

func newConfig(cmd *cli.Command, opts client.Options, taskQueue string) *Config {
	return &Config{
		SlackChannelNamePrefix:    cmd.String("slack-channel-name-prefix"),
		SlackChannelNameMaxLength: cmd.Int("slack-channel-name-max-length"),
		SlackChannelsArePrivate:   cmd.Bool("slack-private-channels"),

		LinkifyMap: config.KVSliceToMap(cmd.StringSlice("linkification-map")),

		Opts: opts,
		TQ:   taskQueue,
	}
}

type (
	prWorkflowFunc   func(workflow.Context, bitbucket.PullRequestEvent) error
	repoWorkflowFunc func(workflow.Context, bitbucket.RepositoryEvent) error
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

// Schedules is a list of workflow names that RevChat runs periodically via
// Temporal schedules (https://docs.temporal.io/develop/go/schedules).
var Schedules = []string{
	"bitbucket.schedules.poll_comment",
}

// RegisterPullRequestWorkflows maps event-handling workflow functions to [PullRequestSignals].
func RegisterPullRequestWorkflows(cmd *cli.Command, opts client.Options, taskQueue string, w worker.Worker) {
	c := newConfig(cmd, opts, taskQueue)
	funcs := []prWorkflowFunc{
		c.PullRequestCreatedWorkflow,
		c.PullRequestUpdatedWorkflow,
		PullRequestReviewedWorkflow, // Approved.
		PullRequestReviewedWorkflow, // Unapproved.
		PullRequestReviewedWorkflow, // Changes requested.
		PullRequestReviewedWorkflow, // Changes request removed.
		PullRequestClosedWorkflow,   // Fulfilled, a.k.a. merged.
		PullRequestClosedWorkflow,   // Rejected, a.k.a. declined.

		c.CommentCreatedWorkflow,
		c.CommentUpdatedWorkflow,
		c.CommentDeletedWorkflow,
		CommentResolvedWorkflow,
		CommentReopenedWorkflow,
	}
	for i, f := range funcs {
		w.RegisterWorkflowWithOptions(f, workflow.RegisterOptions{Name: PullRequestSignals[i]})
	}

	// Special case: scheduled workflows.
	w.RegisterWorkflowWithOptions(c.PollCommentWorkflow, workflow.RegisterOptions{Name: Schedules[0]})
}

// RegisterRepositoryWorkflows maps event-handling workflow functions to [RepositorySignals].
func RegisterRepositoryWorkflows(w worker.Worker) {
	funcs := []repoWorkflowFunc{
		CommitCommentCreatedWorkflow,

		CommitStatusWorkflow,
		CommitStatusWorkflow,
	}
	for i, f := range funcs {
		w.RegisterWorkflowWithOptions(f, workflow.RegisterOptions{Name: RepositorySignals[i]})
	}
}

// RegisterPullRequestSignals routes [PullRequestSignals] to their registered workflows.
func RegisterPullRequestSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	for _, signalName := range PullRequestSignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
			payload := new(bitbucket.PullRequestEvent)
			ch.Receive(ctx, payload)

			signal := ch.Name()
			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.pullrequest.")
			logger.From(ctx).Info("received signal", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)

			// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
			opts := workflow.ChildWorkflowOptions{
				TaskQueue:  taskQueue,
				WorkflowID: prChildWorkflowID(ctx, payload),
			}
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

func prChildWorkflowID(ctx workflow.Context, event *bitbucket.PullRequestEvent) string {
	id := event.PullRequest.Links["html"].HRef
	if event.Comment != nil {
		id = event.Comment.Links["html"].HRef
	}
	id = strings.TrimPrefix(id, "https://bitbucket.org/")

	var ts int64
	encoded := workflow.SideEffect(ctx, func(_ workflow.Context) any {
		return time.Now().UnixMilli()
	})
	if err := encoded.Get(&ts); err != nil {
		return "" // This should never happen, but just in case: let Temporal use its own default.
	}
	return fmt.Sprintf("%s_%s", id, strconv.FormatInt(ts, 36))
}

// RegisterRepositorySignals routes [RepositorySignals] to their registered workflows.
func RegisterRepositorySignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	for _, signalName := range RepositorySignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
			payload := new(bitbucket.RepositoryEvent)
			ch.Receive(ctx, payload)

			signal := ch.Name()
			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.repo.")
			logger.From(ctx).Info("received signal", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)

			// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
				WorkflowID:        repoChildWorkflowID(ctx, payload),
				TaskQueue:         taskQueue,
			})
			_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).GetChildWorkflowExecution().Get(ctx, nil)
		})
	}
}

func repoChildWorkflowID(ctx workflow.Context, event *bitbucket.RepositoryEvent) string {
	id := fmt.Sprintf("%s_%s", event.Repository.FullName, event.Actor.AccountID)
	if event.CommitStatus != nil {
		id = strings.TrimPrefix(event.CommitStatus.Commit.Links["html"].HRef, "https://bitbucket.org/")
	}

	var ts int64
	encoded := workflow.SideEffect(ctx, func(_ workflow.Context) any {
		return time.Now().UnixMilli()
	})
	if err := encoded.Get(&ts); err != nil {
		return "" // This should never happen, but just in case: let Temporal use its own default.
	}
	return fmt.Sprintf("%s_%s", id, strconv.FormatInt(ts, 36))
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
			payload := new(bitbucket.PullRequestEvent)
			if !ch.ReceiveAsync(payload) {
				break
			}

			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.pullrequest.")
			logger.From(ctx).Info("received signal while draining", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)
			signalEvents++

			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
			_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).Get(ctx, nil)
		}

		if signalEvents > 0 {
			logger.From(ctx).Info("drained signal channel",
				slog.String("signal", signal), slog.Int("event_count", signalEvents))
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
			payload := new(bitbucket.RepositoryEvent)
			if !ch.ReceiveAsync(payload) {
				break
			}

			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.repo.")
			logger.From(ctx).Info("received signal while draining", slog.String("signal", signal))
			metrics.IncrementSignalCounter(ctx, signal)
			signalEvents++

			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
			_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).Get(ctx, nil)
		}

		if signalEvents > 0 {
			logger.From(ctx).Info("drained signal channel",
				slog.String("signal", signal), slog.Int("event_count", signalEvents))
		}
		totalEvents += signalEvents
	}
	return totalEvents > 0
}
