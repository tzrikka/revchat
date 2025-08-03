package revchat

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/temporal"
	"github.com/tzrikka/revchat/pkg/data"
)

// Signals is a list of all the Temporal signals that
// [GitHub.EventsWorkflow] may receive from [Timpani].
//
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners/github
var Signals = []string{
	"github.events.pull_request",
	"github.events.pull_request_review",
	"github.events.pull_request_review_comment",
	"github.events.pull_request_review_thread",
}

// EventsWorkflow is an always-running Temporal workflow that
// receives all types of [PR events] as signals from [Timpani].
//
// [PR events]: https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
// [Timpani]: https://pkg.go.dev/github.com/tzrikka/timpani/pkg/listeners/github
func (g GitHub) EventsWorkflow(ctx workflow.Context) error {
	ch, err := temporal.GetSignalChannels(ctx, Signals)
	if err != nil {
		return err
	}

	selector := workflow.NewSelector(ctx)
	l := workflow.GetLogger(ctx)
	more := true

	selector.AddReceive(ch[0], func(c workflow.ReceiveChannel, _ bool) {
		event := PullRequestEvent{}
		more = c.Receive(ctx, &event)
		l.Debug("received signal", "signal", ch[0].Name(), "action", event.Action)
		g.handlePullRequestEvent(ctx, event)
	})

	// selector.AddReceive(ch[1], func(c workflow.ReceiveChannel, _ bool) {
	// 	event := PullRequestReviewEvent{}
	// 	more = c.Receive(ctx, &event)
	// 	l.Debug("received signal", "signal", ch[1].Name(), "action", event.Action)
	// 	g.handlePullRequestReviewEvent(ctx, event)
	// })

	// selector.AddReceive(ch[2], func(c workflow.ReceiveChannel, _ bool) {
	// 	event := PullRequestReviewCommentEvent{}
	// 	more = c.Receive(ctx, &event)
	// 	l.Debug("received signal", "signal", ch[2].Name(), "action", event.Action)
	// 	g.handlePullRequestReviewCommentEvent(ctx, event)
	// })

	// selector.AddReceive(ch[3], func(c workflow.ReceiveChannel, _ bool) {
	// 	event := PullRequestReviewThreadEvent{}
	// 	more = c.Receive(ctx, &event)
	// 	l.Debug("received signal", "signal", ch[3].Name(), "action", event.Action)
	// 	g.handlePullRequestReviewThreadEvent(ctx, event)
	// })

	for more {
		selector.Select(ctx)
	}

	return nil
}

func (g GitHub) InitChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	channel, err := g.createChannel(ctx, event.PullRequest)
	if err != nil {
		return err
	}

	// Cosmetics.
	g.setChannelTopic(ctx, channel, event.PullRequest)
	g.setChannelDescription(ctx, channel, event.PullRequest)
	g.postIntroMessage(ctx, channel, event)

	// Map between the GitHub PR and the Slack channel ID, for 2-way event syncs.
	l := workflow.GetLogger(ctx)
	url := event.PullRequest.HTMLURL
	if err := data.MapURLToChannel(url, channel); err != nil {
		msg := "failed to map GitHub PR URL to Slack channel"
		l.Error(msg, "error", err.Error(), "channel", channel, "url", url)
		return err
	}

	return nil
}

func (g GitHub) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	a := g.executeRevChatWorkflow(ctx, "slack.createChannel", pr)

	var channel string
	if err := a.Get(ctx, &channel); err != nil {
		user := "TODO"

		g.executeTimpaniActivity(ctx, ChatPostMessageActivity, ChatPostMessageRequest{
			Channel:      user,
			MarkdownText: "Failed to create Slack channel for " + pr.HTMLURL,
		})

		return "", err
	}

	return channel, nil
}

const (
	maxMetadataLen = 250
)

func (g GitHub) setChannelTopic(ctx workflow.Context, channel string, pr PullRequest) {
	url := pr.HTMLURL
	if len(url) > maxMetadataLen {
		url = url[:maxMetadataLen-4] + " ..."
	}

	req := ConversationsSetTopicRequest{Channel: channel, Topic: url}
	a := g.executeTimpaniActivity(ctx, ConversationsSetTopicActivity, req)
	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel topic"
		workflow.GetLogger(ctx).Error(msg, "error", err.Error(), "channel", channel, "url", pr.HTMLURL)
	}
}

func (g GitHub) setChannelDescription(ctx workflow.Context, channel string, pr PullRequest) {
	title := fmt.Sprintf("`%s`", pr.Title)
	if len(title) > maxMetadataLen {
		title = title[:maxMetadataLen-4] + "`..."
	}

	req := ConversationsSetPurposeRequest{Channel: channel, Purpose: title}
	a := g.executeTimpaniActivity(ctx, ConversationsSetPurposeActivity, req)
	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel description"
		workflow.GetLogger(ctx).Error(msg, "error", err.Error(), "channel", channel, "url", pr.HTMLURL)
	}
}

func (g GitHub) postIntroMessage(ctx workflow.Context, channel string, event PullRequestEvent) {
	action := event.Action
	if action == "ready_for_review" {
		action = "marked as ready for review"
	}

	pr := event.PullRequest
	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.HTMLURL, pr.Title)
	if pr.Body != nil && strings.TrimSpace(*pr.Body) != "" {
		msg += "\n\n"
	}

	g.mentionUserInMessage(ctx, channel, event.Sender, msg)
}

func (g GitHub) mentionUserInMessage(ctx workflow.Context, channel string, user User, msg string) (string, error) {
	msg = fmt.Sprintf(msg, user.Login)

	req := ChatPostMessageRequest{Channel: channel, MarkdownText: msg}
	a := g.executeTimpaniActivity(ctx, ChatPostMessageActivity, req)
	resp := ChatPostMessageResponse{}
	if err := a.Get(ctx, &resp); err != nil {
		msg := "failed to post Slack message"
		workflow.GetLogger(ctx).Error(msg, "error", err.Error(), "channel", channel)
		return "", err
	}

	return resp.TS, nil
}
