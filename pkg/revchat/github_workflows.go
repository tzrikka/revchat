package revchat

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/temporal"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
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
		e := PullRequestEvent{}
		more = c.Receive(ctx, &e)
		l.Debug("received signal", "signal", ch[0].Name(), "action", e.Action)
		g.handlePullRequestEvent(ctx, e)
	})

	// selector.AddReceive(ch[1], func(c workflow.ReceiveChannel, _ bool) {
	// 	e := PullRequestReviewEvent{}
	// 	more = c.Receive(ctx, &e)
	// 	l.Debug("received signal", "signal", ch[1].Name(), "action", e.Action)
	// 	g.handlePullRequestReviewEvent(ctx, e)
	// })

	// selector.AddReceive(ch[2], func(c workflow.ReceiveChannel, _ bool) {
	// 	e := PullRequestReviewCommentEvent{}
	// 	more = c.Receive(ctx, &e)
	// 	l.Debug("received signal", "signal", ch[2].Name(), "action", e.Action)
	// 	g.handlePullRequestReviewCommentEvent(ctx, e)
	// })

	// selector.AddReceive(ch[3], func(c workflow.ReceiveChannel, _ bool) {
	// 	e := PullRequestReviewThreadEvent{}
	// 	more = c.Receive(ctx, &e)
	// 	l.Debug("received signal", "signal", ch[3].Name(), "action", e.Action)
	// 	g.handlePullRequestReviewThreadEvent(ctx, e)
	// })

	for more {
		selector.Select(ctx)
	}

	return nil
}

func (g GitHub) InitChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	channel, err := g.createChannel(ctx, event.PullRequest, event.Sender)
	if err != nil {
		return err
	}

	// Channel cosmetics.
	url := event.PullRequest.HTMLURL
	g.setChannelTopic(ctx, channel, url)
	g.setChannelDescription(ctx, channel, event.PullRequest.Title, url)
	g.postIntroMessage(ctx, channel, event.Action, event, event.Sender)

	// Map between the GitHub PR and the Slack channel ID, for 2-way event syncs.
	l := workflow.GetLogger(ctx)
	if err := data.MapURLToChannel(url, channel); err != nil {
		msg := "failed to map GitHub PR URL to Slack channel"
		l.Error(msg, "error", err.Error(), "channel", channel, "url", url)
		return err
	}

	return nil
}

func (g GitHub) createChannel(ctx workflow.Context, pr PullRequest, sender User) (string, error) {
	a := g.executeRevChatWorkflow(ctx, "slack.createChannel", pr)

	var channel string
	if err := a.Get(ctx, &channel); err != nil {
		user := sender.Login

		msg := "Failed to create Slack channel for " + pr.HTMLURL
		req := slack.ChatPostMessageRequest{Channel: user, MarkdownText: msg}
		g.executeTimpaniActivity(ctx, slack.ChatPostMessageActivity, req)

		return "", err
	}

	return channel, nil
}

const (
	maxMetadataLen = 250
)

func (g GitHub) setChannelTopic(ctx workflow.Context, channel, url string) {
	t := url
	if len(t) > maxMetadataLen {
		t = t[:maxMetadataLen-4] + " ..."
	}

	req := slack.ConversationsSetTopicRequest{Channel: channel, Topic: t}
	a := g.executeTimpaniActivity(ctx, slack.ConversationsSetTopicActivity, req)
	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel topic"
		workflow.GetLogger(ctx).Error(msg, "error", err.Error(), "channel", channel, "url", url)
	}
}

func (g GitHub) setChannelDescription(ctx workflow.Context, channel, title, url string) {
	t := fmt.Sprintf("`%s`", title)
	if len(t) > maxMetadataLen {
		t = t[:maxMetadataLen-4] + "`..."
	}

	req := slack.ConversationsSetPurposeRequest{Channel: channel, Purpose: t}
	a := g.executeTimpaniActivity(ctx, slack.ConversationsSetPurposeActivity, req)
	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel description"
		workflow.GetLogger(ctx).Error(msg, "error", err.Error(), "channel", channel, "url", url)
	}
}

func (g GitHub) postIntroMessage(ctx workflow.Context, channel, action string, event PullRequestEvent, sender User) {
	if action == "ready_for_review" {
		action = "marked as ready for review"
	}

	pr := event.PullRequest
	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.HTMLURL, pr.Title)
	if pr.Body != nil && strings.TrimSpace(*pr.Body) != "" {
		msg += "\n\n"
	}

	_, _ = g.mentionUserInMessage(ctx, channel, sender, msg)
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
