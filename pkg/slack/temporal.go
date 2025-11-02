package slack

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

type Config struct {
	bitbucketWorkspace string

	thrippyGRPCAddress        string
	thrippyHTTPAddress        string
	thrippyClientCert         string
	thrippyClientKey          string
	thrippyServerCACert       string
	thrippyServerNameOverride string

	thrippyLinksTemplate     string
	thrippyLinksClientID     string
	thrippyLinksClientSecret string
}

func newConfig(cmd *cli.Command) *Config {
	return &Config{
		bitbucketWorkspace: cmd.String("bitbucket-workspace"),

		thrippyGRPCAddress:        cmd.String("thrippy-grpc-address"),
		thrippyHTTPAddress:        cmd.String("thrippy-http-address"),
		thrippyClientCert:         cmd.String("thrippy-client-cert"),
		thrippyClientKey:          cmd.String("thrippy-client-key"),
		thrippyServerCACert:       cmd.String("thrippy-server-ca-cert"),
		thrippyServerNameOverride: cmd.String("thrippy-server-name-override"),
	}
}

// Signals is a list of signal names that RevChat receives
// from Timpani, to trigger event handling workflows.
//
// This is based on:
//   - https://docs.slack.dev/reference/events?APIs=Events
//   - https://github.com/tzrikka/revchat/blob/main/docs/slack.md#bot-event-subscriptions
//   - https://github.com/tzrikka/timpani/blob/main/pkg/listeners/slack/dispatch.go
var Signals = []string{
	"slack.events.app_rate_limited",

	"slack.events.member_joined_channel",
	"slack.events.member_left_channel",
	"slack.events.message",
	"slack.events.reaction_added",
	"slack.events.reaction_removed",

	"slack.events.slash_command",
}

// Schedules is a list of workflow names that RevChat runs periodically via
// Temporal schedules (https://docs.temporal.io/develop/go/schedules).
var Schedules = []string{
	"slack.schedules.reminders",
}

// RegisterWorkflows maps event handler functions to [Signals].
func RegisterWorkflows(ctx context.Context, w worker.Worker, cmd *cli.Command) {
	c := newConfig(cmd)
	c.initThrippyLinks(ctx, cmd.String("thrippy-links-template-id"))

	w.RegisterWorkflowWithOptions(c.appRateLimitedWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(c.memberJoinedWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(c.memberLeftWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(c.messageWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(c.reactionAddedWorkflow, workflow.RegisterOptions{Name: Signals[4]})
	w.RegisterWorkflowWithOptions(c.reactionRemovedWorkflow, workflow.RegisterOptions{Name: Signals[5]})
	w.RegisterWorkflowWithOptions(c.slashCommandWorkflow, workflow.RegisterOptions{Name: Signals[6]})

	w.RegisterWorkflowWithOptions(remindersWorkflow, workflow.RegisterOptions{Name: Schedules[0]})
}

// RegisterSignals routes [Signals] to registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})

	addSignalHandler[map[string]any](sel, ctx, childCtx, Signals[0])
	addSignalHandler[memberEventWrapper](sel, ctx, childCtx, Signals[1])
	addSignalHandler[memberEventWrapper](sel, ctx, childCtx, Signals[2])
	addSignalHandler[messageEventWrapper](sel, ctx, childCtx, Signals[3])
	addSignalHandler[reactionEventWrapper](sel, ctx, childCtx, Signals[4])
	addSignalHandler[reactionEventWrapper](sel, ctx, childCtx, Signals[5])
	addSignalHandler[SlashCommandEvent](sel, ctx, childCtx, Signals[6])
}

func addSignalHandler[T any](sel workflow.Selector, ctx, childCtx workflow.Context, signal string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, signal), func(c workflow.ReceiveChannel, _ bool) {
		var event T
		c.Receive(ctx, &event)

		log.Info(ctx, "received signal", "signal", c.Name())
		workflow.ExecuteChildWorkflow(childCtx, c.Name(), event)
	})
}

// CreateSchedule starts a scheduled workflow that runs every 30 minutes.
func CreateSchedule(ctx context.Context, c client.Client, cmd *cli.Command) {
	_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: Schedules[0],
		Spec: client.ScheduleSpec{
			Calendars: []client.ScheduleCalendarSpec{
				{
					Minute:    []client.ScheduleRange{{Start: 0, End: 59, Step: 30}}, // Every 30 minutes.
					Hour:      []client.ScheduleRange{{Start: 0, End: 23}},
					DayOfWeek: []client.ScheduleRange{{Start: 1, End: 5}}, // Monday to Friday.
				},
			},
			Jitter: 10 * time.Second,
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  Schedules[0],
			TaskQueue: cmd.String("temporal-task-queue-revchat"),
		},
	})
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("id", Schedules[0]).Msg("schedule creation error")
	}
}
