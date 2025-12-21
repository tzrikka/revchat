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
// signals from [Timpani] and spawns event-specific child workflows to handle them.
//
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners
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

		// https://docs.temporal.io/develop/go/continue-as-new
		// https://docs.temporal.io/develop/go/message-passing#wait-for-message-handlers
		if info := workflow.GetInfo(ctx); info.GetContinueAsNewSuggested() {
			startTime := time.Now()
			logger.Info(ctx, "continue-as-new suggested by Temporal server",
				slog.Int("history_length", info.GetCurrentHistoryLength()),
				slog.Int("history_size", info.GetCurrentHistorySize()))

			// "Lame duck" mode: drain all signal channels before resetting workflow history.
			// This minimizes - but doesn't entirely eliminate - the chance of losing signals.
			// We run in this mode until the worker is relatively idle.
			for cyclesSinceLastSignal := 0; cyclesSinceLastSignal < 5; cyclesSinceLastSignal++ {
				if err := workflow.Sleep(ctx, time.Second); err != nil {
					logger.Error(ctx, "failed to wait 1 second between drain cycles", err)
				}
				if c.drainCycle(ctx) {
					cyclesSinceLastSignal = -1 // Will become 0 after loop increment.
				}
			}

			duration := time.Since(startTime)
			logger.Warn(ctx, "triggering continue-as-new for dispatcher workflow",
				slog.Int("history_length", info.GetCurrentHistoryLength()),
				slog.Int("history_size", info.GetCurrentHistorySize()),
				slog.String("lead_time", duration.String()))

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
