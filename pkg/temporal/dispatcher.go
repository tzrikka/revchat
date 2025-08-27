package temporal

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/bitbucket"
)

type config struct {
	cmd *cli.Command
}

// eventsDispatcherWorkflow is an always-running workflow that receives Temporal
// signals from [Timpani] and spawns event-specific child workflows to handle them.
//
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners
func (c config) eventsDispatcherWorkflow(ctx workflow.Context) error {
	s := workflow.NewSelector(ctx)
	tq := c.cmd.String("temporal-task-queue-revchat")

	if err := bitbucket.RegisterPullRequestSignals(ctx, s, tq); err != nil {
		return err
	}
	if err := bitbucket.RegisterRepositorySignals(ctx, s, tq); err != nil {
		return err
	}

	for {
		s.Select(ctx)
	}
}
