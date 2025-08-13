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

	"bitbucket.events.repo.build_status_created",
	"bitbucket.events.repo.build_status_updated",
}

// eventsWorkflow is an always-running Temporal workflow that handles
// all types of [PR events], which are received as signals from [Timpani].
//
// [PR events]: https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners/bitbucket
func (b Bitbucket) eventsWorkflow(ctx workflow.Context) error {
	chs, err := temporal.GetSignalChannels(ctx, Signals)
	if err != nil {
		return err
	}

	selector := workflow.NewSelector(ctx)
	l := workflow.GetLogger(ctx)
	more := true

	for i := range 13 {
		selector.AddReceive(chs[i], func(c workflow.ReceiveChannel, _ bool) {
			e := PullRequestEvent{}
			more = c.Receive(ctx, &e)

			var found bool
			signal := chs[i].Name()
			if _, e.Type, found = strings.Cut(signal, "pullrequest."); !found {
				e.Type = signal
			}

			l.Debug("received signal", "signal", signal)
			b.handlePullRequestEvent(ctx, e)
		})
	}

	for i := 13; i < len(Signals); i++ {
		selector.AddReceive(chs[i], func(c workflow.ReceiveChannel, _ bool) {
			e := RepositoryEvent{}
			more = c.Receive(ctx, &e)

			var found bool
			signal := chs[i].Name()
			if _, e.Type, found = strings.Cut(signal, "repo."); !found {
				e.Type = signal
			}

			l.Debug("received signal", "signal", signal)
			b.handleRepositoryEvent(ctx, e)
		})
	}

	for more {
		selector.Select(ctx)
	}

	return nil
}
