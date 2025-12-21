package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/lmittmann/tint"
	"github.com/urfave/cli/v3"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/temporal"
)

func main() {
	bi, _ := debug.ReadBuildInfo()

	cmd := &cli.Command{
		Name:    "revchat",
		Usage:   "Manage code reviews in dedicated chat channels",
		Version: bi.Main.Version,
		Flags:   config.Flags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			initLog(cmd.Bool("dev") || cmd.Bool("pretty-log"))
			return temporal.Run(ctx, cmd)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// initLog initializes the logger for RevChat's Temporal worker,
// based on whether it's running in development mode or not.
func initLog(devMode bool) {
	var handler slog.Handler
	if devMode {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: "15:04:05.000",
			AddSource:  true,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		})
	}

	slog.SetDefault(slog.New(handler))
}
