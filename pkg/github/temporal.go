package github

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/metrics"
)

type Config struct {
	Cmd *cli.Command
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
	c := Config{Cmd: cmd}
	w.RegisterWorkflowWithOptions(c.pullRequestWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(c.prReviewWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(c.prReviewCommentWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(c.prReviewThreadWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(c.issueCommentWorkflow, workflow.RegisterOptions{Name: Signals[4]})
}

// RegisterSignals routes [Signals] to their registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[0]), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(PullRequestEvent)
		ch.Receive(ctx, payload)
		signal := ch.Name()

		log.Info(ctx, "received signal", "signal", signal, "action", payload.Action)
		metrics.IncrementSignalCounter(ctx, signal)

		wf := workflow.ExecuteChildWorkflow(childCtx, signal, payload)

		// Wait for [Config.prOpened] completion before returning, to ensure we handle
		// subsequent PR initialization events appropriately (e.g. check states).
		if payload.Action == "opened" {
			_ = wf.Get(ctx, nil)
		}
	})

	addReceive[PullRequestReviewEvent](ctx, childCtx, sel, Signals[1])
	addReceive[PullRequestReviewCommentEvent](ctx, childCtx, sel, Signals[2])
	addReceive[PullRequestReviewThreadEvent](ctx, childCtx, sel, Signals[3])
	addReceive[IssueCommentEvent](ctx, childCtx, sel, Signals[4])
}

func addReceive[T any](ctx, childCtx workflow.Context, sel workflow.Selector, signalName string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(T)
		ch.Receive(ctx, payload)
		signal := ch.Name()

		log.Info(ctx, "received signal", "signal", signal)
		metrics.IncrementSignalCounter(ctx, signal)

		workflow.ExecuteChildWorkflow(childCtx, signal, payload)
	})
}

// DrainSignals drains all pending [Signals] channels, starts their
// corresponding workflows as usual, and returns true if any signals were found.
// This is called in preparation for resetting the dispatcher workflow's history.
func DrainSignals(ctx workflow.Context, taskQueue string) bool {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
	totalEvents := 0

	for {
		payload := new(PullRequestEvent)
		if !workflow.GetSignalChannel(ctx, Signals[0]).ReceiveAsync(payload) {
			break
		}

		log.Info(ctx, "received signal while draining", "signal", Signals[0])
		metrics.IncrementSignalCounter(ctx, Signals[0])
		totalEvents++

		wf := workflow.ExecuteChildWorkflow(childCtx, Signals[0], payload)

		// Wait for [Config.prOpened] completion before returning, to ensure we handle
		// subsequent PR initialization events appropriately (e.g. check states).
		if payload.Action == "opened" {
			_ = wf.Get(ctx, nil)
		}
	}
	if totalEvents > 0 {
		log.Info(ctx, "drained signal channel", "signal_name", Signals[0], "event_count", totalEvents)
	}

	totalEvents += receiveAsync[PullRequestReviewEvent](ctx, childCtx, Signals[1])
	totalEvents += receiveAsync[PullRequestReviewCommentEvent](ctx, childCtx, Signals[2])
	totalEvents += receiveAsync[PullRequestReviewThreadEvent](ctx, childCtx, Signals[3])
	totalEvents += receiveAsync[IssueCommentEvent](ctx, childCtx, Signals[4])

	return totalEvents > 0
}

func receiveAsync[T any](ctx, childCtx workflow.Context, signal string) int {
	signalEvents := 0
	for {
		payload := new(T)
		if !workflow.GetSignalChannel(ctx, signal).ReceiveAsync(payload) {
			break
		}

		log.Info(ctx, "received signal while draining", "signal", signal)
		metrics.IncrementSignalCounter(ctx, signal)
		signalEvents++

		workflow.ExecuteChildWorkflow(childCtx, signal, payload)
	}

	if signalEvents > 0 {
		log.Info(ctx, "drained signal channel", "signal_name", signal, "event_count", signalEvents)
	}
	return signalEvents
}
