package github

import (
	"reflect"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/temporal"
)

type Config struct {
	Cmd *cli.Command
}

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
//
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
func RegisterSignals(ctx workflow.Context, s workflow.Selector, tq string) error {
	ch, err := temporal.GetSignalChannels(ctx, Signals)
	if err != nil {
		return err
	}

	m := map[workflow.ReceiveChannel]any{
		ch[0]: PullRequestEvent{},
		ch[1]: PullRequestReviewEvent{},
		ch[2]: PullRequestReviewCommentEvent{},
		ch[3]: PullRequestReviewThreadEvent{},

		ch[4]: IssueCommentEvent{},
	}

	for ch, e := range m {
		s.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, &e)

			action := ""
			f := reflect.ValueOf(e).FieldByName("Action")
			if f.IsValid() && f.Kind() == reflect.String {
				action = f.String()
			}

			signal := c.Name()
			workflow.GetLogger(ctx).Debug("received signal", "signal", signal, "action", action)
			ctx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: tq})
			wf := workflow.ExecuteChildWorkflow(ctx, signal, e)

			// Wait for [Config.prOpened] completion before returning, to ensure we handle
			// subsequent PR initialization events appropriately (e.g. check states).
			if signal == "github.events.pull_request" && action == "opened" {
				_ = wf.Get(ctx, nil)
			}
		})
	}

	return nil
}
