package temporal

import (
	"fmt"
	"slices"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/github"
)

type config struct {
	cmd *cli.Command
}

// eventDispatcherWorkflow is an always-running workflow that receives Temporal
// signals from [Timpani] and spawns event-specific child workflows to handle them.
//
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners
func (c config) eventDispatcherWorkflow(ctx workflow.Context) error {
	// https://docs.temporal.io/develop/go/observability#visibility
	sig := slices.Concat(bitbucket.PullRequestSignals, bitbucket.RepositorySignals, github.Signals)
	attr := temporal.NewSearchAttributeKeyKeywordList("WaitingForSignals").ValueSet(sig)
	if err := workflow.UpsertTypedSearchAttributes(ctx, attr); err != nil {
		return fmt.Errorf("failed to set workflow search attribute: %w", err)
	}

	sel := workflow.NewSelector(ctx)
	tq := c.cmd.String("temporal-task-queue-revchat")
	bitbucket.RegisterPullRequestSignals(ctx, sel, tq)
	bitbucket.RegisterRepositorySignals(ctx, sel, tq)
	github.RegisterSignals(ctx, sel, tq)

	for {
		sel.Select(ctx)
	}
}
