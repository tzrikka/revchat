package github

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type Config struct {
	Cmd *cli.Command
}

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment
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

		workflow.GetLogger(ctx).Debug("received signal", "signal", c.Name(), "action", e.Action)
		wf := workflow.ExecuteChildWorkflow(childCtx, c.Name(), e)

		// Wait for [Config.prOpened] completion before returning, to ensure we handle
		// subsequent PR initialization events appropriately (e.g. check states).
		if e.Action == "opened" {
			_ = wf.Get(ctx, nil)
		}
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[1]), func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestReviewEvent{}
		c.Receive(ctx, &e)

		workflow.GetLogger(ctx).Debug("received signal", "signal", c.Name(), "action", e.Action)
		workflow.ExecuteChildWorkflow(childCtx, c.Name(), e)
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[2]), func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestReviewCommentEvent{}
		c.Receive(ctx, &e)

		workflow.GetLogger(ctx).Debug("received signal", "signal", c.Name(), "action", e.Action)
		workflow.ExecuteChildWorkflow(childCtx, c.Name(), e)
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[3]), func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestReviewThreadEvent{}
		c.Receive(ctx, &e)

		workflow.GetLogger(ctx).Debug("received signal", "signal", c.Name(), "action", e.Action)
		workflow.ExecuteChildWorkflow(childCtx, c.Name(), e)
	})

	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[4]), func(c workflow.ReceiveChannel, _ bool) {
		e := IssueCommentEvent{}
		c.Receive(ctx, &e)

		workflow.GetLogger(ctx).Debug("received signal", "signal", c.Name(), "action", e.Action)
		workflow.ExecuteChildWorkflow(childCtx, c.Name(), e)
	})
}
