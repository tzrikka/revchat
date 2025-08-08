package bitbucket

import (
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/temporal"
)

// Signals enumerates all the [Timpani] signals that this package can handle.
//
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners/bitbucket
var Signals = []string{
	"bitbucket.events.pullrequest.created",
	"bitbucket.events.pullrequest.updated",
	"bitbucket.events.pullrequest.approved",
	"bitbucket.events.pullrequest.unapproved",
	"bitbucket.events.pullrequest.changes_request_created",
	"bitbucket.events.pullrequest.changes_request_removed",
	"bitbucket.events.pullrequest.fulfilled", // a.k.a. merged.
	"bitbucket.events.pullrequest.rejected",  // a.k.a. declined.

	"bitbucket.events.pullrequest.comment_created",
	"bitbucket.events.pullrequest.comment_updated",
	"bitbucket.events.pullrequest.comment_deleted",
	"bitbucket.events.pullrequest.comment_resolved",
	"bitbucket.events.pullrequest.comment_reopened",

	"bitbucket.events.repo.commit_comment_created",
}

// eventsWorkflow is an always-running Temporal workflow that handles
// all types of PR events, which are received as signals from Timpani.
func (b Bitbucket) eventsWorkflow(ctx workflow.Context) error {
	chs, err := temporal.GetSignalChannels(ctx, Signals)
	if err != nil {
		return err
	}

	selector := workflow.NewSelector(ctx)
	l := workflow.GetLogger(ctx)
	more := true

	for _, ch := range chs {
		selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
			e := PullRequestEvent{}
			more = c.Receive(ctx, &e)

			var found bool
			signal := ch.Name()
			if _, e.Type, found = strings.Cut(signal, "events."); !found {
				e.Type = signal
			}

			l.Debug("received signal", "signal", signal)
			b.handlePullRequestEvent(ctx, e)
		})
	}

	for more {
		selector.Select(ctx)
	}

	return nil
}
