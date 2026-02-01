package workflows

import (
	"context"
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
	"github.com/tzrikka/revchat/internal/otel"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/config"
)

type Config struct {
	SlackAlertsChannel        string
	SlackChannelNamePrefix    string
	SlackChannelNameMaxLength int
	SlackChannelsArePrivate   bool

	LinkifyMap map[string]string

	Opts      client.Options
	TaskQueue string
}

func newConfig(cmd *cli.Command, opts client.Options, taskQueue string) *Config {
	return &Config{
		SlackAlertsChannel:        cmd.String("slack-alerts-channel"),
		SlackChannelNamePrefix:    cmd.String("slack-channel-name-prefix"),
		SlackChannelNameMaxLength: cmd.Int("slack-channel-name-max-length"),
		SlackChannelsArePrivate:   cmd.Bool("slack-private-channels"),

		LinkifyMap: config.KVSliceToMap(cmd.StringSlice("linkification-map")),

		Opts:      opts,
		TaskQueue: taskQueue,
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
	"bitbucket.schedules.polling_cleanup",
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
		c.PullRequestClosedWorkflow, // Fulfilled, a.k.a. merged.
		c.PullRequestClosedWorkflow, // Rejected, a.k.a. declined.

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
	w.RegisterWorkflowWithOptions(c.PollingCleanupWorkflow, workflow.RegisterOptions{Name: Schedules[1]})
}

// RegisterRepositoryWorkflows maps event-handling workflow functions to [RepositorySignals].
func RegisterRepositoryWorkflows(cmd *cli.Command, opts client.Options, taskQueue string, w worker.Worker) {
	c := newConfig(cmd, opts, taskQueue)
	funcs := []repoWorkflowFunc{
		CommitCommentCreatedWorkflow,

		c.CommitStatusWorkflow,
		c.CommitStatusWorkflow,
	}
	for i, f := range funcs {
		w.RegisterWorkflowWithOptions(f, workflow.RegisterOptions{Name: RepositorySignals[i]})
	}
}

// RegisterPullRequestSignals routes [PullRequestSignals] to their registered workflows.
func RegisterPullRequestSignals(ctx workflow.Context, sel workflow.Selector) {
	for _, signalName := range PullRequestSignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
			payload := new(bitbucket.PullRequestEvent)
			ch.Receive(ctx, payload)

			signal := ch.Name()
			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.pullrequest.")
			otel.SignalReceived(ctx, signal, false)

			// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
			opts := workflow.ChildWorkflowOptions{WorkflowID: prChildWorkflowID(ctx, payload)}
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
func RegisterRepositorySignals(ctx workflow.Context, sel workflow.Selector) {
	for _, signalName := range RepositorySignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
			payload := new(bitbucket.RepositoryEvent)
			ch.Receive(ctx, payload)

			signal := ch.Name()
			payload.Type = strings.TrimPrefix(signal, "bitbucket.events.repo.")
			otel.SignalReceived(ctx, signal, false)

			// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID:        repoChildWorkflowID(ctx, payload),
				ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
			})
			_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).GetChildWorkflowExecution().Get(ctx, nil)
		})
	}
}

// DrainPullRequestSignals drains all pending [PullRequestSignals] channels,
// and waits for their corresponding workflow executions to complete in order.
// This is called in preparation for resetting the dispatcher workflow's history.
func DrainPullRequestSignals(ctx workflow.Context) bool {
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
			otel.SignalReceived(ctx, signal, true)
			signalEvents++

			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: prChildWorkflowID(ctx, payload),
			})
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
func DrainRepositorySignals(ctx workflow.Context) bool {
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
			otel.SignalReceived(ctx, signal, true)
			signalEvents++

			ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: repoChildWorkflowID(ctx, payload),
			})
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
	return fmt.Sprintf("%s__%s", id, strconv.FormatInt(ts, 36))
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
		return id // This should never happen, but just in case: let Temporal use its own default.
	}
	return fmt.Sprintf("%s__%s", id, strconv.FormatInt(ts, 36))
}

// CreateSchedule starts a scheduled workflow that runs once an hour,
// to delete obsolete (i.e. completed) PR comment polling schedules (which were started by
// [Config.pollCommentForUpdates]) instead of waiting a week for the Temporal server to do it.
func CreateSchedule(ctx context.Context, c client.Client, taskQueue string) {
	_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: Schedules[1],
		Spec: client.ScheduleSpec{
			Calendars: []client.ScheduleCalendarSpec{
				{
					Minute: []client.ScheduleRange{{Start: 15, End: 45, Step: 30}}, // Every 30 minutes.
					Hour:   []client.ScheduleRange{{Start: 0, End: 23}},
				},
			},
			Jitter: 10 * time.Second,
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  Schedules[1],
			TaskQueue: taskQueue,
		},
	})
	if err != nil {
		logger.FromContext(ctx).Warn("failed to initialize Bitbucket comment polling cleanup schedule",
			slog.Any("error", err), slog.String("schedule_id", Schedules[1]))
	}
}
