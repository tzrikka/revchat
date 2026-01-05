package workflows

import (
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/github"
	"github.com/tzrikka/revchat/pkg/metrics"
)

type Config struct {
	SlackChannelNamePrefix    string
	SlackChannelNameMaxLength int
	SlackChannelsArePrivate   bool

	LinkifyMap map[string]string
}

func newConfig(cmd *cli.Command) *Config {
	return &Config{
		SlackChannelNamePrefix:    cmd.String("slack-channel-name-prefix"),
		SlackChannelNameMaxLength: cmd.Int("slack-channel-name-max-length"),
		SlackChannelsArePrivate:   cmd.Bool("slack-private-channels"),

		LinkifyMap: config.KVSliceToMap(cmd.StringSlice("linkification-map")),
	}
}

// Signals is a list of signal names that RevChat receives
// from Timpani, to trigger event handling workflows.
//
// This is based on:
//   - https://docs.github.com/en/webhooks/webhook-events-and-payloads
//   - https://github.com/tzrikka/revchat/blob/main/docs/setup/github.md#subscribe-to-event
//   - https://github.com/tzrikka/timpani/blob/main/pkg/listeners/github/webhook.go
var Signals = []string{
	"github.events.pull_request",
	"github.events.pull_request_review",
	"github.events.pull_request_review_comment",
	"github.events.pull_request_review_thread",

	"github.events.issue_comment",
}

// RegisterWorkflows maps event-handling workflow functions to [Signals].
func RegisterWorkflows(cmd *cli.Command, w worker.Worker) {
	c := newConfig(cmd)
	w.RegisterWorkflowWithOptions(c.PullRequestWorkflow, workflow.RegisterOptions{Name: Signals[0]})
	w.RegisterWorkflowWithOptions(PullRequestReviewWorkflow, workflow.RegisterOptions{Name: Signals[1]})
	w.RegisterWorkflowWithOptions(PullRequestReviewCommentWorkflow, workflow.RegisterOptions{Name: Signals[2]})
	w.RegisterWorkflowWithOptions(PullRequestReviewThreadWorkflow, workflow.RegisterOptions{Name: Signals[3]})
	w.RegisterWorkflowWithOptions(IssueCommentWorkflow, workflow.RegisterOptions{Name: Signals[4]})
}

// RegisterSignals routes [Signals] to their registered workflows.
func RegisterSignals(ctx workflow.Context, sel workflow.Selector) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, Signals[0]), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(github.PullRequestEvent)
		ch.Receive(ctx, payload)

		signal := ch.Name()
		logger.From(ctx).Info("received signal", slog.String("signal", signal), slog.String("action", payload.Action))
		metrics.IncrementSignalCounter(ctx, signal)

		// https://docs.temporal.io/develop/go/child-workflows#parent-close-policy
		opts := workflow.ChildWorkflowOptions{
			WorkflowID: childWorkflowID(ctx, signal, payload),
		}
		if payload.Action != "opened" {
			opts.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
		}
		ctx = workflow.WithChildOptions(ctx, opts)
		wf := workflow.ExecuteChildWorkflow(ctx, signal, payload)

		// Wait for [Config.prOpened] completion before returning, to ensure we handle
		// subsequent PR initialization events appropriately (e.g. check states).
		if payload.Action == "opened" {
			_ = wf.Get(ctx, nil) // Blocks until child workflow completes.
		} else {
			_ = wf.GetChildWorkflowExecution().Get(ctx, nil) // Blocks until child workflow starts.
		}
	})

	addReceive[github.PullRequestReviewEvent](ctx, sel, Signals[1])
	addReceive[github.PullRequestReviewCommentEvent](ctx, sel, Signals[2])
	addReceive[github.PullRequestReviewThreadEvent](ctx, sel, Signals[3])
	addReceive[github.IssueCommentEvent](ctx, sel, Signals[4])
}

func addReceive[T any](ctx workflow.Context, sel workflow.Selector, signalName string) {
	sel.AddReceive(workflow.GetSignalChannel(ctx, signalName), func(ch workflow.ReceiveChannel, _ bool) {
		payload := new(T)
		ch.Receive(ctx, payload)

		signal := ch.Name()
		logger.From(ctx).Info("received signal", slog.String("signal", signal))
		metrics.IncrementSignalCounter(ctx, signal)

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
	totalEvents := receiveAsync[github.PullRequestEvent](ctx, Signals[0])
	totalEvents += receiveAsync[github.PullRequestReviewEvent](ctx, Signals[1])
	totalEvents += receiveAsync[github.PullRequestReviewCommentEvent](ctx, Signals[2])
	totalEvents += receiveAsync[github.PullRequestReviewThreadEvent](ctx, Signals[3])
	totalEvents += receiveAsync[github.IssueCommentEvent](ctx, Signals[4])
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
	case Signals[0]:
		if event, ok := any(payload).(*github.PullRequestEvent); ok {
			id = fmt.Sprintf("%s_%s", event.Action, trimURLPrefix(event.PullRequest.HTMLURL))
		}
	case Signals[1]:
		if event, ok := any(payload).(*github.PullRequestReviewEvent); ok {
			id = fmt.Sprintf("%s_%s", event.Action, trimURLPrefix(event.Review.HTMLURL))
		}
	case Signals[2]:
		if event, ok := any(payload).(*github.PullRequestReviewCommentEvent); ok {
			id = fmt.Sprintf("%s_%s", event.Action, trimURLPrefix(event.Comment.HTMLURL))
		}
	case Signals[3]:
		if event, ok := any(payload).(*github.PullRequestReviewThreadEvent); ok {
			id = fmt.Sprintf("%s_%s", event.Action, trimURLPrefix("TODO"))
		}
	case Signals[4]:
		if event, ok := any(payload).(*github.IssueCommentEvent); ok {
			id = fmt.Sprintf("%s_%s", event.Action, trimURLPrefix(event.Comment.HTMLURL))
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
		return id // This should never happen, but just in case.
	}
	return fmt.Sprintf("%s__%s", id, strconv.FormatInt(ts, 36))
}

func trimURLPrefix(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	suffix := u.Path
	if u.RawQuery != "" {
		suffix += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		suffix += "#" + u.Fragment
	}

	return strings.TrimPrefix(suffix, "/")
}
