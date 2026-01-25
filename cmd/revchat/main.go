package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/lmittmann/tint"
	"github.com/urfave/cli/v3"

	"github.com/tzrikka/revchat/internal/otel"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/temporal"
)

func main() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("Error reading build info")
		os.Exit(1)
	}

	cmd := &cli.Command{
		Name:    "revchat",
		Usage:   "Manage code reviews in dedicated chat channels",
		Version: bi.Main.Version,
		Flags:   config.Flags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			initLog(cmd.Bool("dev"), cmd.Bool("pretty-log"), bi)

			mp, err := otel.InitMetrics(ctx, cmd)
			if err != nil {
				return err
			}
			defer func() {
				if mp != nil {
					_ = mp.Shutdown(ctx)
				}
			}()

			return temporal.Run(ctx, cmd, bi)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// initLog initializes the logger for RevChat's Temporal worker,
// based on whether it's running in development mode or not.
func initLog(dev, prettyLog bool, bi *debug.BuildInfo) {
	var handler slog.Handler
	switch {
	case dev: // Including dev && prettyLog.
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: "15:04:05.000",
			AddSource:  true,
		})
	case prettyLog: // But not dev.
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelInfo,
			TimeFormat: "15:04:05.000",
			AddSource:  true,
		})
	default: // Production JSON log.
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		})
	}

	slog.SetDefault(slog.New(handler))
	slog.Info("build versions", slog.String("go", bi.GoVersion), slog.String("main", bi.Main.Version))
}
