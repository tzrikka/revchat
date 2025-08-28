package bitbucket

import (
	"strings"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

type Config struct {
	Cmd *cli.Command
}

type (
	prWorkflowFunc   func(workflow.Context, PullRequestEvent) error
	repoWorkflowFunc func(workflow.Context, RepositoryEvent) error
)

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Pull-request-events
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

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Repository-events
var RepositorySignals = []string{
	"bitbucket.events.repo.commit_comment_created",

	"bitbucket.events.repo.build_status_created",
	"bitbucket.events.repo.build_status_updated",
}

// RegisterPullRequestWorkflows maps event handler functions to [PullRequestSignals].
func RegisterPullRequestWorkflows(w worker.Worker, cmd *cli.Command) {
	c := Config{Cmd: cmd}
	fs := []prWorkflowFunc{
		c.prCreatedWorkflow,
		c.prUpdatedWorkflow,
		c.prReviewedWorkflow, // Approved.
		c.prReviewedWorkflow, // Unapproved.
		c.prReviewedWorkflow, // Changes requested.
		c.prReviewedWorkflow, // Changes request removed.
		c.prClosedWorkflow,   // Fulfilled, a.k.a. merged.
		c.prClosedWorkflow,   // Rejected, a.k.a. declined.

		c.prCommentCreatedWorkflow,
		c.prCommentUpdatedWorkflow,
		c.prCommentDeletedWorkflow,
		c.prCommentResolvedWorkflow,
		c.prCommentReopenedWorkflow,
	}

	for i, f := range fs {
		w.RegisterWorkflowWithOptions(f, workflow.RegisterOptions{Name: PullRequestSignals[i]})
	}
}

// RegisterRepositoryWorkflows maps event handler functions to [RepositorySignals].
func RegisterRepositoryWorkflows(w worker.Worker, cmd *cli.Command) {
	c := Config{Cmd: cmd}
	fs := []repoWorkflowFunc{
		c.commitCommentCreatedWorkflow,

		c.buildStatusCreatedWorkflow,
		c.buildStatusUpdatedWorkflow,
	}

	for i, f := range fs {
		w.RegisterWorkflowWithOptions(f, workflow.RegisterOptions{Name: RepositorySignals[i]})
	}
}

// RegisterPullRequestSignals routes [PullRequestSignals] to registered workflows.
func RegisterPullRequestSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	for _, sig := range PullRequestSignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, sig), func(c workflow.ReceiveChannel, _ bool) {
			e := PullRequestEvent{}
			c.Receive(ctx, &e)

			var found bool
			signal := c.Name()
			if _, e.Type, found = strings.Cut(signal, "pullrequest."); !found {
				e.Type = signal
			}

			log.Info(ctx, "received signal", "signal", signal)
			childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
			wf := workflow.ExecuteChildWorkflow(childCtx, signal, e)

			// Wait for [Config.prCreated] completion before returning, to ensure we handle
			// subsequent PR initialization events appropriately (e.g. check states).
			if signal == "bitbucket.events.pullrequest.created" {
				_ = wf.Get(ctx, nil)
			}
		})
	}
}

// RegisterRepositorySignals routes [RepositorySignals] to registered workflows.
func RegisterRepositorySignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	for _, sig := range RepositorySignals {
		sel.AddReceive(workflow.GetSignalChannel(ctx, sig), func(c workflow.ReceiveChannel, _ bool) {
			e := RepositoryEvent{}
			c.Receive(ctx, &e)

			var found bool
			signal := c.Name()
			if _, e.Type, found = strings.Cut(signal, "repo."); !found {
				e.Type = signal
			}

			log.Info(ctx, "received signal", "signal", signal)
			childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})
			workflow.ExecuteChildWorkflow(childCtx, signal, e)
		})
	}
}
