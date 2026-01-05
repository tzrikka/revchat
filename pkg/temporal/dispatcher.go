package temporal

import (
	"fmt"
	"log/slog"
	"slices"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/workflows"
	github "github.com/tzrikka/revchat/pkg/github/workflows"
	slack "github.com/tzrikka/revchat/pkg/slack/workflows"
)

const (
	// SearchAttribute is a Temporal search attribute key used
	// by Timpani and the RevChat event dispatcher workflow.
	SearchAttribute = "WaitingForSignals"

	// EventDispatcher is the name and ID of the RevChat event dispatcher workflow.
	EventDispatcher = "event.dispatcher"

	shutdownSignal = "graceful.shutdown"
)

type Config struct {
	dispatcherWorkflowID string
	dispatcherRunID      string

	shutdownSignal string
	shutdownDone   chan<- any
}

// EventDispatcherWorkflow is an always-running singleton workflow that receives Temporal
// signals from [Timpani] and spawns event-specific child workflows to handle them. Most
// of these child workflows run asynchronously, with the exception of PR creation events.
//
// This workflow enters a brief "lame duck" mode before adhering to the Temporal server's
// [continue-as-new] suggestions, or OS signals to stop the worker process, in order to
// ensure that no events are lost during history resets.
//
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners
// [continue-as-new]: https://docs.temporal.io/develop/go/continue-as-new
func (c *Config) EventDispatcherWorkflow(ctx workflow.Context) error {
	// https://docs.temporal.io/develop/go/observability#visibility
	sig := slices.Concat(bitbucket.PullRequestSignals, bitbucket.RepositorySignals, github.Signals, slack.Signals)
	attr := temporal.NewSearchAttributeKeyKeywordList(SearchAttribute).ValueSet(sig)
	if err := workflow.UpsertTypedSearchAttributes(ctx, attr); err != nil {
		return fmt.Errorf("failed to set workflow search attribute: %w", err)
	}

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(workflow.GetSignalChannel(ctx, shutdownSignal), func(ch workflow.ReceiveChannel, _ bool) {
		ch.Receive(ctx, &c.shutdownSignal)
	})

	bitbucket.RegisterPullRequestSignals(ctx, selector)
	bitbucket.RegisterRepositorySignals(ctx, selector)
	github.RegisterSignals(ctx, selector)
	slack.RegisterSignals(ctx, selector)

	for {
		selector.Select(ctx) // This is the core of the dispatcher workflow's event loop.

		info := workflow.GetInfo(ctx)
		c.dispatcherWorkflowID = info.WorkflowExecution.ID
		c.dispatcherRunID = info.WorkflowExecution.RunID

		// "Lame duck" mode.
		continueAsNewSuggested := info.GetContinueAsNewSuggested()
		if continueAsNewSuggested || c.shutdownSignal != "" {
			c.prepareForReset(ctx, info)
		}

		switch {
		case continueAsNewSuggested:
			return workflow.NewContinueAsNewError(ctx, EventDispatcher)

		case c.shutdownSignal != "":
			encoded := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
				c.shutdownDone <- "done"
				return ""
			})
			_ = encoded.Get(&c.shutdownSignal)
		}
	}
}

// prepareForReset waits for all child workflows to finish and drains all signal channels.
func (c *Config) prepareForReset(ctx workflow.Context, info *workflow.Info) {
	startTime := time.Now()

	// https://docs.temporal.io/develop/go/message-passing#wait-for-message-handlers
	err := workflow.Await(ctx, func() bool {
		return workflow.AllHandlersFinished(ctx)
	})
	if err != nil {
		logger.From(ctx).Error("failed to wait for all handlers to finish", slog.Any("error", err))
	}

	for cyclesSinceLastSignal := 0; cyclesSinceLastSignal < 5; cyclesSinceLastSignal++ {
		if drainCycle(ctx) {
			cyclesSinceLastSignal = -1 // Will become 0 after loop increment.
		}
	}

	msg := "triggering continue-as-new for dispatcher workflow"
	if c.shutdownSignal != "" {
		msg = "shutting down worker process as requested by OS signal"
	}
	logger.From(ctx).Warn(msg, slog.Duration("lead_time", time.Since(startTime)),
		slog.Int("history_length", info.GetCurrentHistoryLength()),
		slog.Int("history_size", info.GetCurrentHistorySize()))
}

// drainCycle processes each event source and returns true if any signals were found.
func drainCycle(ctx workflow.Context) bool {
	bitbucketPRSignalsFound := bitbucket.DrainPullRequestSignals(ctx)
	bitbucketRepoSignalsFound := bitbucket.DrainRepositorySignals(ctx)
	githubSignalsFound := github.DrainSignals(ctx)
	slackSignalsFound := slack.DrainSignals(ctx)

	return bitbucketPRSignalsFound || bitbucketRepoSignalsFound || githubSignalsFound || slackSignalsFound
}
