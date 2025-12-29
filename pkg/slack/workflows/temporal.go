package workflows

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/metrics"
	"github.com/tzrikka/revchat/pkg/slack/commands"
)

type Config struct {
	BitbucketWorkspace string

	ThrippyGRPCAddress        string
	ThrippyHTTPAddress        string
	thrippyClientCert         string
	thrippyClientKey          string
	thrippyServerCACert       string
	thrippyServerNameOverride string

	ThrippyLinksTemplate     string
	thrippyLinksClientID     string
	thrippyLinksClientSecret string
}

func newConfig(cmd *cli.Command) *Config {
	return &Config{
		BitbucketWorkspace: cmd.String("bitbucket-workspace"),

		ThrippyGRPCAddress:        cmd.String("thrippy-grpc-address"),
		ThrippyHTTPAddress:        cmd.String("thrippy-http-address"),
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

	"slack.events.channel_archive",
	"slack.events.group_archive",
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
func RegisterWorkflows(ctx context.Context, cmd *cli.Command, w worker.Worker) {
	c := newConfig(cmd)
	c.initThrippyLinks(ctx, cmd.String("thrippy-links-template-id"))

	w.RegisterWorkflowWithOptions(AppRateLimitedWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(ChannelArchivedWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(ChannelArchivedWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(MemberJoinedWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(MemberLeftWorkflow, workflow.RegisterOptions{Name: Signals[4]})
	w.RegisterWorkflowWithOptions(c.MessageWorkflow, workflow.RegisterOptions{Name: Signals[5]})
	w.RegisterWorkflowWithOptions(ReactionAddedWorkflow, workflow.RegisterOptions{Name: Signals[6]})
	w.RegisterWorkflowWithOptions(ReactionRemovedWorkflow, workflow.RegisterOptions{Name: Signals[7]})
	w.RegisterWorkflowWithOptions(c.SlashCommandWorkflow, workflow.RegisterOptions{Name: Signals[8]})

	// Special case: scheduled workflows.
	w.RegisterWorkflowWithOptions(RemindersWorkflow, workflow.RegisterOptions{Name: Schedules[0]})
}

// RegisterSignals routes [Signals] to their registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector) {
	addReceive[map[string]any](ctx, sel, Signals[0])
	addReceive[archiveEventWrapper](ctx, sel, Signals[1])
	addReceive[archiveEventWrapper](ctx, sel, Signals[2])
	addReceive[memberEventWrapper](ctx, sel, Signals[3])
	addReceive[memberEventWrapper](ctx, sel, Signals[4])
	addReceive[messageEventWrapper](ctx, sel, Signals[5])
	addReceive[reactionEventWrapper](ctx, sel, Signals[6])
	addReceive[reactionEventWrapper](ctx, sel, Signals[7])
	addReceive[commands.SlashCommandEvent](ctx, sel, Signals[8])
}

func addReceive[T any](ctx workflow.Context, sel workflow.Selector, signalName string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(T)
		ch.Receive(ctx, payload)

		signal := ch.Name()
		logger.From(ctx).Info("received signal", slog.String("signal", signal))
		metrics.IncrementSignalCounter(ctx, signal)

		// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
		ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID:        childWorkflowID(ctx, signal, payload),
			ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
		})
		_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).GetChildWorkflowExecution().Get(ctx, nil)
	})
}

// DrainSignals drains all pending [Signals] channels, and waits
// for their corresponding workflow executions to complete in order.
// This is called in preparation for resetting the dispatcher workflow's history.
func DrainSignals(ctx workflow.Context) bool {
	totalEvents := receiveAsync[map[string]any](ctx, Signals[0])
	totalEvents += receiveAsync[archiveEventWrapper](ctx, Signals[1])
	totalEvents += receiveAsync[archiveEventWrapper](ctx, Signals[2])
	totalEvents += receiveAsync[memberEventWrapper](ctx, Signals[3])
	totalEvents += receiveAsync[memberEventWrapper](ctx, Signals[4])
	totalEvents += receiveAsync[messageEventWrapper](ctx, Signals[5])
	totalEvents += receiveAsync[reactionEventWrapper](ctx, Signals[6])
	totalEvents += receiveAsync[reactionEventWrapper](ctx, Signals[7])
	totalEvents += receiveAsync[commands.SlashCommandEvent](ctx, Signals[8])
	return totalEvents > 0
}

func receiveAsync[T any](ctx workflow.Context, signal string) int {
	ch := workflow.GetSignalChannel(ctx, signal)
	signalEvents := 0
	for {
		payload := new(T)
		if !ch.ReceiveAsync(payload) {
			break
		}

		logger.From(ctx).Info("received signal while draining", slog.String("signal", signal))
		metrics.IncrementSignalCounter(ctx, signal)
		signalEvents++

		ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: childWorkflowID(ctx, signal, payload),
		})
		_ = workflow.ExecuteChildWorkflow(ctx, signal, payload).Get(ctx, nil)
	}

	if signalEvents > 0 {
		logger.From(ctx).Info("drained signal channel", slog.String("signal", signal),
			slog.Int("event_count", signalEvents))
	}
	return signalEvents
}

func childWorkflowID[T any](ctx workflow.Context, signal string, payload *T) string {
	id := ""
	switch signal {
	case Signals[1], Signals[2]:
		if event, ok := any(payload).(*archiveEventWrapper); ok {
			id = event.InnerEvent.Channel
		}
	case Signals[3], Signals[4]:
		if event, ok := any(payload).(*memberEventWrapper); ok {
			id = fmt.Sprintf("%s_%s", event.InnerEvent.Channel, event.InnerEvent.User)
		}
	case Signals[5]:
		if event, ok := any(payload).(*messageEventWrapper); ok {
			subtype := "created"
			if event.InnerEvent.Subtype != "" {
				subtype = event.InnerEvent.Subtype
			}
			id = fmt.Sprintf("%s_%s_%s", subtype, event.InnerEvent.Channel, event.InnerEvent.TS)
		}
	case Signals[6], Signals[7]:
		if event, ok := any(payload).(*reactionEventWrapper); ok {
			e := event.InnerEvent
			id = fmt.Sprintf("%s_%s", e.Item.Channel, e.Item.TS)
		}
	case Signals[8]:
		if event, ok := any(payload).(*commands.SlashCommandEvent); ok {
			cmd := ""
			if text := strings.TrimSpace(event.Text); text != "" {
				cmd = strings.SplitN(text, " ", 2)[0] + "_"
			}
			id = fmt.Sprintf("%s%s_%s", cmd, event.ChannelID, event.UserID)
		}
	}

	if id == "" {
		return "" // Fallback in case of unexpected payloads: let Temporal use its own default.
	}

	var ts int64
	encoded := workflow.SideEffect(ctx, func(_ workflow.Context) any {
		return time.Now().UnixMilli()
	})
	if err := encoded.Get(&ts); err != nil {
		return id // This should never happen, but just in case: let Temporal use its own default.
	}
	return fmt.Sprintf("%s_%s", id, strconv.FormatInt(ts, 36))
}

// CreateSchedule starts a scheduled workflow that runs every 30 minutes, to send daily reminders.
func CreateSchedule(ctx context.Context, c client.Client, taskQueue string) {
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
			TaskQueue: taskQueue,
		},
	})
	if err != nil {
		logger.FromContext(ctx).Warn("failed to initialize reminders schedule",
			slog.Any("error", err), slog.String("schedule_id", Schedules[0]))
	}
}
