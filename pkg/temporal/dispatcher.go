package temporal

import (
	"fmt"
	"slices"
	"time"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
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
	cmd *cli.Command
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
	taskQueue := c.cmd.String("temporal-task-queue-revchat")
	bitbucket.RegisterPullRequestSignals(ctx, selector, taskQueue)
	bitbucket.RegisterRepositorySignals(ctx, selector, taskQueue)
	github.RegisterSignals(ctx, selector, taskQueue)
	slack.RegisterSignals(ctx, selector, taskQueue)

	for {
		selector.Select(ctx)

		// https://docs.temporal.io/develop/go/continue-as-new
		// https://docs.temporal.io/develop/go/message-passing#wait-for-message-handlers
		if info := workflow.GetInfo(ctx); info.GetContinueAsNewSuggested() {
			startTime := time.Now()
			log.Info(ctx, "continue-as-new suggested by Temporal server",
				"history_length", info.GetCurrentHistoryLength(),
				"history_size", info.GetCurrentHistorySize())

			// "Lame duck" mode: drain all signal channels before resetting workflow history.
			// This minimizes - but doesn't entirely eliminate - the chance of losing signals.
			// We run in this mode until the worker is relatively idle.
			for cyclesSinceLastSignal := 0; cyclesSinceLastSignal < 5; cyclesSinceLastSignal++ {
				if err := workflow.Sleep(ctx, time.Second); err != nil {
					log.Error(ctx, "failed to wait 1 second between drain cycles", "error", err)
				}
				if drainCycle(ctx, taskQueue) {
					cyclesSinceLastSignal = -1 // Will become 0 after loop increment.
				}
			}

			duration := time.Since(startTime)
			log.Warn(ctx, "triggering continue-as-new for dispatcher workflow",
				"history_length", info.GetCurrentHistoryLength(),
				"history_size", info.GetCurrentHistorySize(),
				"lead_time", duration.String())

			return workflow.NewContinueAsNewError(ctx, EventDispatcher)
		}
	}
}

// drainCycle processes each event source and returns true if any signals were found.
func drainCycle(ctx workflow.Context, taskQueue string) bool {
	bitbucketPRSignalsFound := bitbucket.DrainPullRequestSignals(ctx, taskQueue)
	bitbucketRepoSignalsFound := bitbucket.DrainRepositorySignals(ctx, taskQueue)
	githubSignalsFound := github.DrainSignals(ctx, taskQueue)
	slackSignalsFound := slack.DrainSignals(ctx, taskQueue)

	return bitbucketPRSignalsFound || bitbucketRepoSignalsFound || githubSignalsFound || slackSignalsFound
}
