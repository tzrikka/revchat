package slack

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"
)

// NormalizeChannelName transforms arbitrary text into a valid Slack channel name.
// Based on: https://docs.slack.dev/reference/methods/conversations.create#naming.
func NormalizeChannelName(name string, maxLen int) string {
	if name == "" {
		return name
	}

	name = regexp.MustCompile(`\[[\w -]*\]`).ReplaceAllString(name, "") // Remove annotations.

	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	name = regexp.MustCompile("['`]").ReplaceAllString(name, "")          // Remove apostrophes.
	name = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(name, "-") // Replace invalid characters.
	name = regexp.MustCompile(`[_-]{2,}`).ReplaceAllString(name, "-")     // Minimize "-" separators.

	name = strings.TrimPrefix(name, "-")
	name = strings.TrimPrefix(name, "_")

	if len(name) > maxLen {
		name = name[:maxLen]
	}

	name = strings.TrimSuffix(name, "-")
	name = strings.TrimSuffix(name, "_")

	return name
}

const (
	channelMetadataMaxLen = 250
)

// https://docs.slack.dev/reference/methods/conversations.archive
type conversationsArchiveRequest struct {
	Channel string `json:"channel"`
}

// https://docs.slack.dev/reference/methods/conversations.create
type conversationsCreateRequest struct {
	Name string `json:"name"`

	IsPrivate bool   `json:"is_private,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.create
type ConversationsCreateResponse struct {
	slackResponse

	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
}

// https://docs.slack.dev/reference/methods/conversations.invite
type conversationsInviteRequest struct {
	Channel string `json:"channel"`
	Users   string `json:"users"`

	Force bool `json:"force,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.invite
type conversationsInviteResponse struct {
	slackResponse

	Channel map[string]any   `json:"channel,omitempty"`
	Errors  []map[string]any `json:"errors,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.setPurpose
type conversationsSetPurposeRequest struct {
	Channel string `json:"channel"`
	Purpose string `json:"purpose"`
}

// https://docs.slack.dev/reference/methods/conversations.setTopic
type conversationsSetTopicRequest struct {
	Channel string `json:"channel"`
	Topic   string `json:"topic"`
}

// https://docs.slack.dev/reference/methods/conversations.archive
func ArchiveChannelActivity(ctx workflow.Context, cmd *cli.Command, channelID string) error {
	req := conversationsArchiveRequest{Channel: channelID}
	a := executeTimpaniActivity(ctx, cmd, "slack.conversations.archive", req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to archive Slack channel"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID)
		return err
	}

	return nil
}

// https://docs.slack.dev/reference/methods/conversations.create
func CreateChannelActivity(ctx workflow.Context, cmd *cli.Command, name, url string) (string, bool, error) {
	req := conversationsCreateRequest{Name: name, IsPrivate: false}
	a := executeTimpaniActivity(ctx, cmd, "slack.conversations.create", req)
	l := workflow.GetLogger(ctx)

	resp := &ConversationsCreateResponse{}
	if err := a.Get(ctx, resp); err != nil {
		msg := "failed to create Slack channel"

		if strings.Contains(err.Error(), "name_taken") {
			l.Debug(msg+" - already exists", "channel_name", name, "pr_url", url)
			return "", true, err // Retryable error.
		}

		l.Error(msg, "error", err, "channel_name", name, "pr_url", url)
		return "", false, err // Non-retryable error.
	}

	id := resp.Channel.ID
	l.Info("created Slack channel", "channel_id", id, "channel_name", name, "pr_url", url)
	return id, false, nil
}

// https://docs.slack.dev/reference/methods/conversations.invite
func InviteUsersToChannelActivity(ctx workflow.Context, cmd *cli.Command, channelID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	l := workflow.GetLogger(ctx)
	if len(userIDs) > 1000 { // API limitation.
		msg := "trying to add more than 1000 users to Slack channel"
		l.Warn(msg, "channel_id", channelID, "users_len", len(userIDs))
		userIDs = userIDs[:1000]
	}

	ids := strings.Join(userIDs, ",")
	req := conversationsInviteRequest{Channel: channelID, Users: ids, Force: true}
	a := executeTimpaniActivity(ctx, cmd, "slack.conversations.invite", req)

	resp := &conversationsInviteResponse{}
	if err := a.Get(ctx, resp); err != nil {
		msg := "failed to add user(s) to Slack channel"
		if strings.Contains(err.Error(), "already_in_channel") {
			l.Debug(msg+" - already in channel", "error", err, "resp", resp, "users_len", len(userIDs))
			return nil
		}
		l.Error(msg, "error", err, "channel_id", channelID, "users_len", len(userIDs))
		return err
	}

	return nil
}

// https://docs.slack.dev/reference/methods/conversations.setPurpose
func SetChannelDescriptionActivity(ctx workflow.Context, cmd *cli.Command, channelID, title, url string) {
	t := fmt.Sprintf("`%s`", title)
	if len(t) > channelMetadataMaxLen {
		t = t[:channelMetadataMaxLen-4] + "`..."
	}

	req := conversationsSetPurposeRequest{Channel: channelID, Purpose: t}
	a := executeTimpaniActivity(ctx, cmd, "slack.conversations.setPurpose", req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel description"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
	}
}

// https://docs.slack.dev/reference/methods/conversations.setTopic
func SetChannelTopicActivity(ctx workflow.Context, cmd *cli.Command, channelID, url string) {
	t := url
	if len(t) > channelMetadataMaxLen {
		t = t[:channelMetadataMaxLen-4] + " ..."
	}

	req := conversationsSetTopicRequest{Channel: channelID, Topic: t}
	a := executeTimpaniActivity(ctx, cmd, "slack.conversations.setTopic", req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel topic"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
	}
}
