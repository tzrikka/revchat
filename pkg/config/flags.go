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
	MaxRetryAttempts       = 5

	DefaultChannelNamePrefix    = "_pr"
	DefaultMaxChannelNameLength = 50 // Slack's hard limit = 80, but that's still too long.

	DefaultThrippyGRPCAddress = "localhost:14460"
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

		// Bitbucket.
		&cli.StringFlag{
			Name:  "bitbucket-workspace",
			Usage: "Bitbucket workspace slug",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("BITBUCKET_WORKSPACE"),
				toml.TOML("bitbucket.workspace", path),
			),
		},

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
		&cli.BoolFlag{
			Name:  "slack-private-channels",
			Usage: "Make PR channels private",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("SLACK_PRIVATE_CHANNELS"),
				toml.TOML("slack.private_channels", path),
			),
		},

		// Linkification.
		&cli.StringSliceFlag{
			Name:  "linkification-map",
			Usage: `Map of case-sensitive project keys to URL prefixes (e.g. PROJ=https://domain.atlassian.net/browse/, supports "default" key)`,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("LINKIFICATION_MAP"),
				toml.TOML("linkification.map", path),
			),
		},

		// Thrippy.
		&cli.StringFlag{
			Name:  "thrippy-grpc-address",
			Usage: "Thrippy gRPC server address",
			Value: DefaultThrippyGRPCAddress,
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_GRPC_ADDRESS"),
				toml.TOML("thrippy.grpc_address", path),
			),
		},
		&cli.StringFlag{
			Name:  "thrippy-http-address",
			Usage: "Thrippy HTTP server address",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_HTTP_ADDRESS"),
				toml.TOML("thrippy.http_address", path),
			),
			Required: true,
		},
		&cli.StringFlag{
			Name:  "thrippy-client-cert",
			Usage: "Thrippy gRPC client's public certificate PEM file (mTLS only)",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_CLIENT_CERT"),
				toml.TOML("thrippy.client_cert", path),
			),
			TakesFile: true,
		},
		&cli.StringFlag{
			Name:  "thrippy-client-key",
			Usage: "Thrippy gRPC client's private key PEM file (mTLS only)",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_CLIENT_KEY"),
				toml.TOML("thrippy.client_key", path),
			),
			TakesFile: true,
		},
		&cli.StringFlag{
			Name:  "thrippy-server-ca-cert",
			Usage: "Thrippy gRPC server's CA certificate PEM file (both TLS and mTLS)",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_SERVER_CA_CERT"),
				toml.TOML("thrippy.server_ca_cert", path),
			),
			TakesFile: true,
			Required:  true,
		},
		&cli.StringFlag{
			Name:  "thrippy-server-name-override",
			Usage: "Thrippy gRPC server's name override (for testing, both TLS and mTLS)",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_SERVER_NAME_OVERRIDE"),
				toml.TOML("thrippy.server_name_override", path),
			),
		},

		// Thrippy links (for Bitbucket or GitHub).
		&cli.StringFlag{
			Name:  "thrippy-links-template",
			Usage: "Thrippy links template name",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_LINKS_TEMPLATE"),
				toml.TOML("thrippy.links.template", path),
			),
			Required: true,
		},
		&cli.StringFlag{
			Name:  "thrippy-links-client-id",
			Usage: "Thrippy links OAuth client ID",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_LINKS_CLIENT_ID"),
				toml.TOML("thrippy.links.client_id", path),
			),
			Required: true,
		},
		&cli.StringFlag{
			Name:  "thrippy-links-client-secret",
			Usage: "Thrippy links OAuth client secret (Bitbucket only)",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("THRIPPY_LINKS_CLIENT_SECRET"),
				toml.TOML("thrippy.links.client_secret", path),
			),
		},
	}
}
