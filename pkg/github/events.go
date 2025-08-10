package github

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/temporal"
)

// Signals enumerates all the [Timpani] signals that this package can handle.
//
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners/github
var Signals = []string{
	"github.events.pull_request",
	"github.events.pull_request_review",
	"github.events.pull_request_review_comment",
	"github.events.pull_request_review_thread",
}

// eventsWorkflow is an always-running Temporal workflow that handles
// all types of [PR events], which are received as signals from [Timpani].
//
// [PR events]: https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners/github
func (g GitHub) eventsWorkflow(ctx workflow.Context) error {
	ch, err := temporal.GetSignalChannels(ctx, Signals)
	if err != nil {
		return err
	}

	selector := workflow.NewSelector(ctx)
	l := workflow.GetLogger(ctx)
	more := true

	selector.AddReceive(ch[0], func(c workflow.ReceiveChannel, _ bool) {
		e := PullRequestEvent{}
		more = c.Receive(ctx, &e)
		l.Debug("received signal", "signal", ch[0].Name(), "action", e.Action)
		g.handlePullRequestEvent(ctx, e)
	})

	// selector.AddReceive(ch[1], func(c workflow.ReceiveChannel, _ bool) {
	// 	e := PullRequestReviewEvent{}
	// 	more = c.Receive(ctx, &e)
	// 	l.Debug("received signal", "signal", ch[1].Name(), "action", e.Action)
	// 	g.handlePullRequestReviewEvent(ctx, e)
	// })

	// selector.AddReceive(ch[2], func(c workflow.ReceiveChannel, _ bool) {
	// 	e := PullRequestReviewCommentEvent{}
	// 	more = c.Receive(ctx, &e)
	// 	l.Debug("received signal", "signal", ch[2].Name(), "action", e.Action)
	// 	g.handlePullRequestReviewCommentEvent(ctx, e)
	// })

	// selector.AddReceive(ch[3], func(c workflow.ReceiveChannel, _ bool) {
	// 	e := PullRequestReviewThreadEvent{}
	// 	more = c.Receive(ctx, &e)
	// 	l.Debug("received signal", "signal", ch[3].Name(), "action", e.Action)
	// 	g.handlePullRequestReviewThreadEvent(ctx, e)
	// })

	for more {
		selector.Select(ctx)
	}

	return nil
}
