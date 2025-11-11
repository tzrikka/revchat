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

	sel := workflow.NewSelector(ctx)
	tq := c.cmd.String("temporal-task-queue-revchat")
	bitbucket.RegisterPullRequestSignals(ctx, sel, tq)
	bitbucket.RegisterRepositorySignals(ctx, sel, tq)
	github.RegisterSignals(ctx, sel, tq)
	slack.RegisterSignals(ctx, sel, tq)

	for {
		sel.Select(ctx)

		// https://docs.temporal.io/develop/go/continue-as-new
		// https://docs.temporal.io/develop/go/message-passing#wait-for-message-handlers
		if info := workflow.GetInfo(ctx); info.GetContinueAsNewSuggested() {
			msg := "continue-as-new suggested by Temporal server"
			log.Info(ctx, msg, "history_length", info.GetCurrentHistoryLength(), "history_size", info.GetCurrentHistorySize())

			// "Lame duck" mode: drain all signal channels before resetting workflow history.
			// This minimizes - but doesn't entirely eliminate - the chance of losing signals.
			// That's why we run this in a slowed-down loop until no signals are left to process:
			// it will continue until the worker is relatively idle.
			counter := 1
			for counter > 0 {
				workflow.Sleep(ctx, 5*time.Second)
				counter = bitbucket.DrainPullRequestSignals(ctx, tq)
				counter += bitbucket.DrainRepositorySignals(ctx, tq)
				counter += github.DrainSignals(ctx, tq)
				counter += slack.DrainSignals(ctx, tq)
			}

			msg = "triggering workflow continue-as-new"
			log.Warn(ctx, msg, "history_length", info.GetCurrentHistoryLength(), "history_size", info.GetCurrentHistorySize())
			return workflow.NewContinueAsNewError(ctx, EventDispatcher)
		}
	}
}
