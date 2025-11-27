package slack

import (
	"fmt"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

const (
	channelMetadataMaxLen = 250
)

// https://docs.slack.dev/reference/events/channel_archive/
// https://docs.slack.dev/reference/events/group_archive/
func (c *Config) channelArchivedWorkflow(ctx workflow.Context, event archiveEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	log.Warn(ctx, "Slack channel archived - event handler not implemented yet")
	return nil
}

// NormalizeChannelName transforms arbitrary text into a valid Slack channel name.
// Based on: https://docs.slack.dev/reference/methods/conversations.create#naming.
func NormalizeChannelName(name string, maxLen int) string {
	if name == "" {
		return name
	}

	name = regexp.MustCompile(`\[[\w -]*\]`).ReplaceAllString(name, "")      // Remove annotations.
	name = regexp.MustCompile(`[A-Z]{3,}-\d{5,}`).ReplaceAllString(name, "") // Remove annotations.

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

func CreateChannel(ctx workflow.Context, name, url string, private bool) (string, bool, error) {
	id, err := slack.ConversationsCreate(ctx, name, private)
	if err != nil {
		msg := "failed to create Slack channel"

		if strings.Contains(err.Error(), "name_taken") {
			log.Debug(ctx, msg+" - already exists", "channel_name", name, "pr_url", url)
			return "", true, err // Retry with a different name.
		}

		log.Error(ctx, msg, "error", err, "channel_name", name, "pr_url", url)
		return "", false, err // Non-retryable error.
	}

	log.Info(ctx, "created Slack channel", "channel_id", id, "channel_name", name, "pr_url", url)
	return id, false, nil
}

func RenameChannel(ctx workflow.Context, channelID, name string) (bool, error) {
	if err := slack.ConversationsRename(ctx, channelID, name); err != nil {
		msg := "failed to rename Slack channel"

		if strings.Contains(err.Error(), "name_taken") {
			log.Debug(ctx, msg+" - already exists", "channel_id", channelID, "new_name", name)
			return true, err // Retry with a different name.
		}

		log.Error(ctx, msg, "error", err, "channel_id", channelID, "new_name", name)
		return false, err // Non-retryable error.
	}

	log.Info(ctx, "renamed Slack channel", "channel_id", channelID, "new_name", name)
	return false, nil
}

func InviteUsersToChannel(ctx workflow.Context, channelID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	if len(userIDs) > 1000 { // API limitation.
		msg := "trying to add more than 1000 users to Slack channel - truncating"
		log.Warn(ctx, msg, "channel_id", channelID, "users_len", len(userIDs))
		userIDs = userIDs[:1000]
	}

	if err := slack.ConversationsInvite(ctx, channelID, userIDs, true); err != nil {
		msg := "failed to add user(s) to Slack channel"

		if strings.Contains(err.Error(), "already_in_channel") {
			msg += " - already in channel" // This is not a problem.
			log.Debug(ctx, msg, "error", err, "channel_id", channelID, "user_ids", strings.Join(userIDs, ","))
			return nil
		}

		log.Error(ctx, msg, "error", err, "channel_id", channelID, "user_ids", strings.Join(userIDs, ","))
		return err
	}

	return nil
}

func KickUsersFromChannel(ctx workflow.Context, channelID string, userIDs []string) error {
	var err error
	for _, userID := range userIDs {
		err = slack.ConversationsKick(ctx, channelID, userID)
		if err != nil {
			msg := "failed to remove user from Slack channel"

			if strings.Contains(err.Error(), "not_in_channel") {
				msg += " - not in channel" // This is not a problem.
				log.Debug(ctx, msg, "error", err, "channel_id", channelID, "user_id", userID)
				continue
			}

			log.Error(ctx, msg, "error", err, "channel_id", channelID, "user_id", userID)
		}
	}

	return err
}

func SetChannelDescription(ctx workflow.Context, channelID, title, url string) {
	desc := fmt.Sprintf("`%s`", title)
	if len(desc) > channelMetadataMaxLen {
		desc = desc[:channelMetadataMaxLen-4] + "`..."
	}

	if err := slack.ConversationsSetPurpose(ctx, channelID, desc); err != nil {
		msg := "failed to set Slack channel description"
		log.Error(ctx, msg, "error", err, "channel_id", channelID, "pr_url", url)
	}
}

func SetChannelTopic(ctx workflow.Context, channelID, url string) {
	topic := url
	if len(topic) > channelMetadataMaxLen {
		topic = topic[:channelMetadataMaxLen-4] + " ..."
	}

	if err := slack.ConversationsSetTopic(ctx, channelID, topic); err != nil {
		log.Error(ctx, "failed to set Slack channel topic", "error", err, "channel_id", channelID, "pr_url", url)
	}
}
