package revchat

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
)

func (s Slack) ArchiveChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	channel, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return nil // If we're not tracking the PR, there's no channel to archive.
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 5*time.Second)

	state := event.Action + " this PR"
	if event.Action == "closed" && event.PullRequest.Merged {
		state = "merged this PR"
	}
	if event.Action == "converted_to_draft" {
		state = "converted this PR to a draft"
	}

	mentionGitHubUserInMessage(ctx, channel, event.Sender, "%s "+state)

	req := ConversationsArchiveRequest{Channel: channel}
	a := s.executeTimpaniActivity(ctx, ConversationsArchiveActivity, req)

	if err := a.Get(ctx, nil); err != nil {
		l := workflow.GetLogger(ctx)
		msg := "failed to archive Slack channel"
		l.Error(msg, "error", err, "action", event.Action, "channel", channel, "url", event.PullRequest.HTMLURL)

		state = strings.Replace(state, " this PR", "", 1)
		msg = fmt.Sprintf("Failed to archive this channel, even though its PR was %s: %s", state, err.Error())
		req := ChatPostMessageRequest{Channel: channel, MarkdownText: msg}
		s.executeTimpaniActivity(ctx, ChatPostMessageActivity, req)
		return err
	}

	return nil
}

func (s Slack) CreateChannelWorkflow(ctx workflow.Context, pr PullRequest) (*string, error) {
	title := slack.NormalizeChannelName(pr.Title, s.cmd.Int("slack-max-channel-name-length"))
	l := workflow.GetLogger(ctx)

	for i := 1; i < 10; i++ {
		name := fmt.Sprintf("%s_%s", s.cmd.String("slack-channel-name-prefix"), title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		req := ConversationsCreateRequest{Name: name}
		a := s.executeTimpaniActivity(ctx, ConversationsCreateActivity, req)

		resp := &ConversationsCreateResponse{}
		if err := a.Get(ctx, resp); err != nil {
			msg := "failed to create Slack channel"
			if !strings.Contains(err.Error(), "name_taken") {
				l.Error(msg, "error", err, "name", name, "url", pr.HTMLURL)
				return nil, temporal.NewNonRetryableApplicationError(msg, fmt.Sprintf("%T", err), err)
			}

			l.Debug(msg+" - already exists", "name", name)
			continue // Retry with a different name.
		}

		channel, ok := resp.Channel["id"]
		if !ok {
			msg := "created Slack channel without ID"
			l.Error(msg, "resp", resp)
			return nil, temporal.NewNonRetryableApplicationError(msg, "error", nil, resp)
		}

		id, ok := channel.(string)
		if !ok || len(id) == 0 {
			msg := "created Slack channel with invalid ID"
			l.Error(msg, "resp", resp)
			return nil, temporal.NewNonRetryableApplicationError(msg, "error", nil, id)
		}

		l.Info("created Slack channel for GitHub PR", "channel_id", id, "channel_name", name, "url", pr.HTMLURL)
		return &id, nil
	}

	msg := "too many failed attempts to create Slack channel for GitHub PR"
	l.Error(msg, "url", pr.HTMLURL)
	return nil, temporal.NewNonRetryableApplicationError(msg, "error", nil, pr.HTMLURL)
}
