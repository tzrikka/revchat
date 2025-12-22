package github

import (
	"log/slog"

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

// Signals is a list of signal names that RevChat receives
// from Timpani, to trigger event handling workflows.
//
// This is based on:
//   - https://github.com/tzrikka/revchat/blob/main/docs/setup/github.md#subscribe-to-event
//   - https://github.com/tzrikka/timpani/blob/main/pkg/listeners/github/webhook.go
var Signals = []string{
	"github.events.pull_request",
	"github.events.pull_request_review",
	"github.events.pull_request_review_comment",
	"github.events.pull_request_review_thread",

	"github.events.issue_comment",
}

// RegisterWorkflows maps event-handling workflow functions to [Signals].
func RegisterWorkflows(w worker.Worker, cmd *cli.Command) {
	c := newConfig(cmd)
	w.RegisterWorkflowWithOptions(c.pullRequestWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(c.prReviewWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(c.prReviewCommentWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(c.prReviewThreadWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(c.issueCommentWorkflow, workflow.RegisterOptions{Name: Signals[4]})
}

// RegisterSignals routes [Signals] to their registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[0]), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(PullRequestEvent)
		ch.Receive(ctx, payload)

		signal := ch.Name()
		logger.Info(ctx, "received signal", slog.String("signal", signal), slog.String("action", payload.Action))
		metrics.IncrementSignalCounter(ctx, signal)

		// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
		opts := workflow.ChildWorkflowOptions{TaskQueue: taskQueue}
		if payload.Action != "opened" {
			opts.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
		}
		ctx = workflow.WithChildOptions(ctx, opts)
		wf := workflow.ExecuteChildWorkflow(ctx, signal, payload)

		// Wait for [Config.prOpened] completion before returning, to ensure we handle
		// subsequent PR initialization events appropriately (e.g. check states).
		if payload.Action == "opened" {
			_ = wf.Get(ctx, nil) // Blocks until child workflow completes.
		} else {
			_ = wf.GetChildWorkflowExecution().Get(ctx, nil) // Blocks until child workflow starts.
		}
	})

	addReceive[PullRequestReviewEvent](ctx, sel, taskQueue, Signals[1])
	addReceive[PullRequestReviewCommentEvent](ctx, sel, taskQueue, Signals[2])
	addReceive[PullRequestReviewThreadEvent](ctx, sel, taskQueue, Signals[3])
	addReceive[IssueCommentEvent](ctx, sel, taskQueue, Signals[4])
}

func addReceive[T any](ctx workflow.Context, sel workflow.Selector, taskQueue, signalName string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(T)
		ch.Receive(ctx, payload)

		signal := ch.Name()
		logger.Info(ctx, "received signal", slog.String("signal", signal))
		metrics.IncrementSignalCounter(ctx, signal)

		ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
			TaskQueue:         taskQueue,
		})
		_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).GetChildWorkflowExecution().Get(ctx, nil)
	})
}

// DrainSignals drains all pending [Signals] channels, and waits
// for their corresponding workflow executions to complete in order.
// This is called in preparation for resetting the dispatcher workflow's history.
func DrainSignals(ctx workflow.Context, taskQueue string) bool {
	totalEvents := receiveAsync[PullRequestEvent](ctx, taskQueue, Signals[0])
	totalEvents += receiveAsync[PullRequestReviewEvent](ctx, taskQueue, Signals[1])
	totalEvents += receiveAsync[PullRequestReviewCommentEvent](ctx, taskQueue, Signals[2])
	totalEvents += receiveAsync[PullRequestReviewThreadEvent](ctx, taskQueue, Signals[3])
	totalEvents += receiveAsync[IssueCommentEvent](ctx, taskQueue, Signals[4])

	return totalEvents > 0
}

func receiveAsync[T any](ctx workflow.Context, taskQueue, signal string) int {
	ch := workflow.GetSignalChannel(ctx, signal)
	signalEvents := 0
	for {
		payload := new(T)
		if !ch.ReceiveAsync(payload) {
			break
		}

		logger.Info(ctx, "received signal while draining", slog.String("signal", signal))
		metrics.IncrementSignalCounter(ctx, signal)
		signalEvents++

		ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
		_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).Get(ctx, nil)
	}

	if signalEvents > 0 {
		logger.Info(ctx, "drained signal channel", slog.String("signal", signal), slog.Int("event_count", signalEvents))
	}
	return signalEvents
}
