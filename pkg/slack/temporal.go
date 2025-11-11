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
	"github.com/tzrikka/revchat/pkg/metrics"
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
//   - https://github.com/tzrikka/revchat/blob/main/docs/setup/slack.md#bot-event-subscriptions
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

// RegisterWorkflows maps event-handling workflow functions to [Signals].
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

// RegisterSignals routes [Signals] to their registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector, taskQueue string) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})

	addReceive[map[string]any](ctx, childCtx, sel, Signals[0])
	addReceive[memberEventWrapper](ctx, childCtx, sel, Signals[1])
	addReceive[memberEventWrapper](ctx, childCtx, sel, Signals[2])
	addReceive[messageEventWrapper](ctx, childCtx, sel, Signals[3])
	addReceive[reactionEventWrapper](ctx, childCtx, sel, Signals[4])
	addReceive[reactionEventWrapper](ctx, childCtx, sel, Signals[5])
	addReceive[SlashCommandEvent](ctx, childCtx, sel, Signals[6])
}

func addReceive[T any](ctx, childCtx workflow.Context, sel workflow.Selector, signalName string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(T)
		ch.Receive(ctx, payload)
		signal := ch.Name()

		log.Info(ctx, "received signal", "signal", signal)
		metrics.IncrementSignalCounter(ctx, signal)

		workflow.ExecuteChildWorkflow(childCtx, signal, payload)
	})
}

// DrainSignals drains all pending [Signals] channels, starts their
// corresponding workflows as usual, and returns the number of drained events.
// This is called in preparation for resetting the dispatcher workflow's history.
func DrainSignals(ctx workflow.Context, taskQueue string) int {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{TaskQueue: taskQueue})

	totalEvents := receiveAsync[map[string]any](ctx, childCtx, Signals[0])
	totalEvents += receiveAsync[memberEventWrapper](ctx, childCtx, Signals[1])
	totalEvents += receiveAsync[memberEventWrapper](ctx, childCtx, Signals[2])
	totalEvents += receiveAsync[messageEventWrapper](ctx, childCtx, Signals[3])
	totalEvents += receiveAsync[reactionEventWrapper](ctx, childCtx, Signals[4])
	totalEvents += receiveAsync[reactionEventWrapper](ctx, childCtx, Signals[5])
	totalEvents += receiveAsync[SlashCommandEvent](ctx, childCtx, Signals[6])

	return totalEvents
}

func receiveAsync[T any](ctx, childCtx workflow.Context, signal string) int {
	signalEvents := 0
	for {
		payload := new(T)
		if !workflow.GetSignalChannel(ctx, signal).ReceiveAsync(payload) {
			break
		}

		log.Info(ctx, "received signal while draining", "signal", signal)
		metrics.IncrementSignalCounter(ctx, signal)
		signalEvents++

		workflow.ExecuteChildWorkflow(childCtx, signal, payload)
	}

	log.Info(ctx, "drained signal channel", "signal_name", signal, "event_count", signalEvents)
	return signalEvents
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
