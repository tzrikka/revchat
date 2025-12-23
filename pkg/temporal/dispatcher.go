package temporal

import (
	"fmt"
	"log/slog"
	"slices"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/github"
	"github.com/tzrikka/revchat/pkg/slack"
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
	taskQueue string

	shutdown string
	done     chan<- any
}

// EventDispatcherWorkflow is an always-running singleton workflow that receives Temporal
// signals from [Timpani] and spawns event-specific child workflows to handle them. Most
// of these child workflows run asynchronously, with the exception of PR creation events.
//
// This workflow also has a "lame duck" mode in which it waits for all message handlers
// to finish and then drains all signal channels before following the Temporal server's
// [continue-as-new] suggestions, to ensure that no events are lost during history reset.
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
		ch.Receive(ctx, &c.shutdown)
	})

	bitbucket.RegisterPullRequestSignals(ctx, selector, c.taskQueue)
	bitbucket.RegisterRepositorySignals(ctx, selector, c.taskQueue)
	github.RegisterSignals(ctx, selector, c.taskQueue)
	slack.RegisterSignals(ctx, selector, c.taskQueue)

	for {
		selector.Select(ctx)

		// "Lame duck" mode: wait for all child workflows to finish, and drain all signal
		// channels, before resetting the workflow history or shutting down the entire worker:
		//   - https://docs.temporal.io/develop/go/message-passing#wait-for-message-handlers
		//   - https://github.com/temporalio/samples-go/tree/main/safe_message_handler
		if info := workflow.GetInfo(ctx); info.GetContinueAsNewSuggested() || c.shutdown != "" {
			startTime := time.Now()

			err := workflow.Await(ctx, func() bool {
				return workflow.AllHandlersFinished(ctx)
			})
			if err != nil {
				logger.From(ctx).Error("failed to wait for all handlers to finish", slog.Any("error", err))
			}

			for cyclesSinceLastSignal := 0; cyclesSinceLastSignal < 5; cyclesSinceLastSignal++ {
				if c.drainCycle(ctx) {
					cyclesSinceLastSignal = -1 // Will become 0 after loop increment.
				}
			}

			msg := "triggering continue-as-new for dispatcher workflow"
			if c.shutdown != "" {
				msg += ", and shutting down the worker"
			}
			logger.From(ctx).Warn(msg, slog.String("lead_time", time.Since(startTime).String()),
				slog.Int("history_length", info.GetCurrentHistoryLength()),
				slog.Int("history_size", info.GetCurrentHistorySize()))

			if c.shutdown != "" {
				c.done <- c.shutdown
				close(c.done)
			}

			return workflow.NewContinueAsNewError(ctx, EventDispatcher)
		}
	}
}

// drainCycle processes each event source and returns true if any signals were found.
func (c *Config) drainCycle(ctx workflow.Context) bool {
	bitbucketPRSignalsFound := bitbucket.DrainPullRequestSignals(ctx, c.taskQueue)
	bitbucketRepoSignalsFound := bitbucket.DrainRepositorySignals(ctx, c.taskQueue)
	githubSignalsFound := github.DrainSignals(ctx, c.taskQueue)
	slackSignalsFound := slack.DrainSignals(ctx, c.taskQueue)

	return bitbucketPRSignalsFound || bitbucketRepoSignalsFound || githubSignalsFound || slackSignalsFound
}
