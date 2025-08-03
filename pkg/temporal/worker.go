// Package temporal initializes a Temporal worker.
package temporal

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/tzrikka/revchat/pkg/revchat"
)

// Run initializes the Temporal worker, and blocks.
func Run(l zerolog.Logger, cmd *cli.Command) error {
	addr := cmd.String("temporal-host-port")
	l.Info().Msgf("Temporal server address: %s", addr)

	c, err := client.Dial(client.Options{
		HostPort:  addr,
		Namespace: cmd.String("temporal-namespace"),
		Logger:    LogAdapter{zerolog: l},
	})
	if err != nil {
		return fmt.Errorf("failed to dial Temporal: %w", err)
	}
	defer c.Close()

	w := worker.New(c, cmd.String("temporal-task-queue-revchat"), worker.Options{})
	revchat.RegisterGitHubWorkflows(w, cmd)
	revchat.RegisterSlackWorkflows(w, cmd)

	if err := w.Run(worker.InterruptCh()); err != nil {
		return fmt.Errorf("failed to start Temporal worker: %w", err)
	}

	return nil
}
