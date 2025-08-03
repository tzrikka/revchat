package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
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
			initLog(cmd.Bool("dev"))
			return temporal.Run(log.Logger, cmd)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// initLog initializes the logger for Timpani's HTTP server and Temporal
// worker, based on whether it's running in development mode or not.
func initLog(devMode bool) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	if !devMode {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
		return
	}

	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05.000",
	}).With().Caller().Logger()
}
