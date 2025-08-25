package config

import (
	"time"

	"github.com/rs/zerolog/log"
	altsrc "github.com/urfave/cli-altsrc/v3"
	"github.com/urfave/cli-altsrc/v3/toml"
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/client"

	"github.com/tzrikka/xdg"
)

const (
	DirName        = "revchat"
	ConfigFileName = "config.toml"

	DefaultRevChatTaskQueue = "revchat"
	DefaultTimpaniTaskQueue = "timpani"

	ScheduleToStartTimeout = time.Minute
	StartToCloseTimeout    = 5 * time.Second
	MaxRetryAttempts       = 3

	DefaultChannelNamePrefix    = "_pr"
	DefaultMaxChannelNameLength = 50 // Slack's hard limit = 80, but that's still too long.
)

// configFile returns the path to the app's configuration file.
// It also creates an empty file if it doesn't already exist.
func configFile() altsrc.StringSourcer {
	path, err := xdg.CreateFile(xdg.ConfigHome, DirName, ConfigFileName)
	if err != nil {
		log.Fatal().Err(err).Caller().Send()
	}
	return altsrc.StringSourcer(path)
}

// Flags defines CLI flags to configure a Temporal worker. These flags are usually
// set using environment variables or the application's configuration file.
func Flags() []cli.Flag {
	path := configFile()

	return []cli.Flag{
		&cli.BoolFlag{
			Name:  "dev",
			Usage: "simple setup, but unsafe for production",
		},
		&cli.BoolFlag{
			Name:  "pretty-log",
			Usage: "human-readable console logging, instead of JSON",
		},

		// https://pkg.go.dev/go.temporal.io/sdk/internal#ClientOptions
		&cli.StringFlag{
			Name:  "temporal-address",
			Usage: "Temporal server address",
			Value: client.DefaultHostPort,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("TEMPORAL_ADDRESS"),
				toml.TOML("temporal.address", path),
			),
		},
		&cli.StringFlag{
			Name:  "temporal-namespace",
			Usage: "Temporal namespace",
			Value: client.DefaultNamespace,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("TEMPORAL_NAMESPACE"),
				toml.TOML("temporal.namespace", path),
			),
		},

		// Worker parameter.
		&cli.StringFlag{
			Name:  "temporal-task-queue-revchat",
			Usage: "Temporal task queue for the RevChat worker",
			Value: DefaultRevChatTaskQueue,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("TEMPORAL_TASK_QUEUE_REVCHAT"),
				toml.TOML("temporal.revchat_task_queue", path),
			),
		},
		&cli.StringFlag{
			Name:  "temporal-task-queue-timpani",
			Usage: "Temporal task queue for the Timpani worker",
			Value: DefaultTimpaniTaskQueue,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("TEMPORAL_TASK_QUEUE_TIMPANI"),
				toml.TOML("temporal.timpani_task_queue", path),
			),
		},

		// https://pkg.go.dev/go.temporal.io/sdk/internal#WorkerOptions

		// Slack.
		&cli.StringFlag{
			Name:  "slack-channel-name-prefix",
			Usage: "Prefix for Slack channel names",
			Value: DefaultChannelNamePrefix,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("SLACK_CHANNEL_NAME_PREFIX"),
				toml.TOML("slack.channel_name_prefix", path),
			),
		},
		&cli.IntFlag{
			Name:  "slack-max-channel-name-length",
			Usage: "Maximum length of Slack channel names",
			Value: DefaultMaxChannelNameLength,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("SLACK_MAX_CHANNEL_NAME_LENGTH"),
				toml.TOML("slack.max_channel_name_length", path),
			),
		},
	}
}
