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

// RegisterWorkflows maps event handler functions to [Signals].
func RegisterWorkflows(w worker.Worker, cmd *cli.Command) {
	c := Config{Cmd: cmd}
	w.RegisterWorkflowWithOptions(c.pullRequestWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(c.prReviewWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(c.prReviewCommentWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(c.prReviewThreadWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(c.issueCommentWorkflow, workflow.RegisterOptions{Name: Signals[4]})
}

// RegisterSignals routes [Signals] to registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[0]), func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestEvent{}
		c.Receive(ctx, &e)

		signal := c.Name()
		log.Info(ctx, "received signal", "signal", signal, "action", e.Action)
		metrics.IncrementSignalCounter(ctx, signal)

		wf := workflow.ExecuteChildWorkflow(childCtx, signal, e)

		// Wait for [Config.prOpened] completion before returning, to ensure we handle
		// subsequent PR initialization events appropriately (e.g. check states).
		if e.Action == "opened" {
			_ = wf.Get(ctx, nil)
		}
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[1]), func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestReviewEvent{}
		c.Receive(ctx, &e)

		signal := c.Name()
		log.Info(ctx, "received signal", "signal", signal, "action", e.Action)
		metrics.IncrementSignalCounter(ctx, signal)

		workflow.ExecuteChildWorkflow(childCtx, signal, e)
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[2]), func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestReviewCommentEvent{}
		c.Receive(ctx, &e)

		signal := c.Name()
		log.Info(ctx, "received signal", "signal", signal, "action", e.Action)
		metrics.IncrementSignalCounter(ctx, signal)

		workflow.ExecuteChildWorkflow(childCtx, signal, e)
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[3]), func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestReviewThreadEvent{}
		c.Receive(ctx, &e)

		signal := c.Name()
		log.Info(ctx, "received signal", "signal", signal, "action", e.Action)
		metrics.IncrementSignalCounter(ctx, signal)

		workflow.ExecuteChildWorkflow(childCtx, signal, e)
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[4]), func(c workflow.ReceiveChannel, _ bool) {
		e := IssueCommentEvent{}
		c.Receive(ctx, &e)

		signal := c.Name()
		log.Info(ctx, "received signal", "signal", signal, "action", e.Action)
		metrics.IncrementSignalCounter(ctx, signal)

		workflow.ExecuteChildWorkflow(childCtx, signal, e)
	})
}
