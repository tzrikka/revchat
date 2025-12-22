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
)

type Config struct {
	taskQueue string
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
func (c Config) EventDispatcherWorkflow(ctx workflow.Context) error {
	// https://docs.temporal.io/develop/go/observability#visibility
	sig := slices.Concat(bitbucket.PullRequestSignals, bitbucket.RepositorySignals, github.Signals, slack.Signals)
	attr := temporal.NewSearchAttributeKeyKeywordList(SearchAttribute).ValueSet(sig)
	if err := workflow.UpsertTypedSearchAttributes(ctx, attr); err != nil {
		return fmt.Errorf("failed to set workflow search attribute: %w", err)
	}

	selector := workflow.NewSelector(ctx)
	bitbucket.RegisterPullRequestSignals(ctx, selector, c.taskQueue)
	bitbucket.RegisterRepositorySignals(ctx, selector, c.taskQueue)
	github.RegisterSignals(ctx, selector, c.taskQueue)
	slack.RegisterSignals(ctx, selector, c.taskQueue)

	for {
		selector.Select(ctx)

		// https://docs.temporal.io/develop/go/message-passing#wait-for-message-handlers
		// https://github.com/temporalio/samples-go/tree/main/safe_message_handler
		if info := workflow.GetInfo(ctx); info.GetContinueAsNewSuggested() {
			startTime := time.Now()
			logger.Info(ctx, "continue-as-new suggested by Temporal server",
				slog.Int("history_length", info.GetCurrentHistoryLength()),
				slog.Int("history_size", info.GetCurrentHistorySize()))

			// "Lame duck" mode: wait for all child workflows to finish, and
			// drain all signal channels, before resetting workflow history.
			err := workflow.Await(ctx, func() bool {
				return workflow.AllHandlersFinished(ctx)
			})
			if err != nil {
				logger.Error(ctx, "failed to wait for all handlers to finish", err)
			}

			logger.Info(ctx, "all child workflow handlers have finished before continue-as-new",
				slog.Int("history_length", info.GetCurrentHistoryLength()),
				slog.Int("history_size", info.GetCurrentHistorySize()),
				slog.String("lead_time", time.Since(startTime).String()))

			for cyclesSinceLastSignal := 0; cyclesSinceLastSignal < 5; cyclesSinceLastSignal++ {
				if c.drainCycle(ctx) {
					cyclesSinceLastSignal = -1 // Will become 0 after loop increment.
				}
			}

			logger.Warn(ctx, "triggering continue-as-new for dispatcher workflow",
				slog.Int("history_length", info.GetCurrentHistoryLength()),
				slog.Int("history_size", info.GetCurrentHistorySize()),
				slog.String("lead_time", time.Since(startTime).String()))

			return workflow.NewContinueAsNewError(ctx, EventDispatcher)
		}
	}
}

// drainCycle processes each event source and returns true if any signals were found.
func (c Config) drainCycle(ctx workflow.Context) bool {
	bitbucketPRSignalsFound := bitbucket.DrainPullRequestSignals(ctx, c.taskQueue)
	bitbucketRepoSignalsFound := bitbucket.DrainRepositorySignals(ctx, c.taskQueue)
	githubSignalsFound := github.DrainSignals(ctx, c.taskQueue)
	slackSignalsFound := slack.DrainSignals(ctx, c.taskQueue)

	return bitbucketPRSignalsFound || bitbucketRepoSignalsFound || githubSignalsFound || slackSignalsFound
}
