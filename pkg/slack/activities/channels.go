package activities

import (
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

const (
	channelMetadataMaxLen = 250
)

// ArchiveChannel is an idempotent function, unlike the underlying Slack API call.
func ArchiveChannel(ctx workflow.Context, channelID, prURL string) error {
	if err := slack.ConversationsArchive(ctx, channelID); err != nil && !strings.Contains(err.Error(), "is_archived") {
		logger.From(ctx).Error("failed to archive Slack channel", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("pr_url", prURL))
		return err
	}
	return nil
}

func CreateChannel(ctx workflow.Context, name, prURL string, private bool) (id string, retry bool, err error) {
	id, err = slack.ConversationsCreate(ctx, name, private)
	if err != nil {
		if strings.Contains(err.Error(), "name_taken") {
			logger.From(ctx).Debug("failed to create Slack channel - already exists",
				slog.String("channel_name", name), slog.String("pr_url", prURL))
			return "", true, err // Retry with a different name.
		}

		logger.From(ctx).Error("failed to create Slack channel", slog.Any("error", err),
			slog.String("channel_name", name), slog.String("pr_url", prURL))
		return "", false, err // Non-retryable error.
	}

	logger.From(ctx).Info("created Slack channel", slog.String("channel_id", id),
		slog.String("channel_name", name), slog.String("pr_url", prURL))
	return id, false, nil
}

func InviteUsersToChannel(ctx workflow.Context, channelID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	if len(userIDs) > 1000 { // API limitation.
		logger.From(ctx).Warn("trying to add more than 1000 users to Slack channel - truncating",
			slog.String("channel_id", channelID), slog.Int("users_len", len(userIDs)))
		userIDs = userIDs[:1000]
	}

	if err := slack.ConversationsInvite(ctx, channelID, userIDs, true); err != nil {
		msg := "failed to add user(s) to Slack channel"

		if strings.Contains(err.Error(), "already_in_channel") {
			msg += " - already in channel" // This is not a problem.
			logger.From(ctx).Debug(msg, slog.Any("error", err), slog.String("channel_id", channelID),
				slog.String("user_ids", strings.Join(userIDs, ",")))
			return nil
		}

		logger.From(ctx).Error(msg, slog.Any("error", err), slog.String("channel_id", channelID),
			slog.String("user_ids", strings.Join(userIDs, ",")))
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
				logger.From(ctx).Debug(msg, slog.Any("error", err),
					slog.String("channel_id", channelID), slog.String("user_id", userID))
				continue
			}

			logger.From(ctx).Error(msg, slog.Any("error", err),
				slog.String("channel_id", channelID), slog.String("user_id", userID))
		}
	}

	return err
}

func RenameChannel(ctx workflow.Context, channelID, name string) (bool, error) {
	if err := slack.ConversationsRename(ctx, channelID, name); err != nil {
		if strings.Contains(err.Error(), "name_taken") {
			logger.From(ctx).Debug("failed to rename Slack channel - already exists",
				slog.String("channel_id", channelID), slog.String("new_name", name))
			return true, err // Retry with a different name.
		}

		logger.From(ctx).Error("failed to rename Slack channel", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("new_name", name))
		return false, err // Non-retryable error.
	}

	logger.From(ctx).Info("renamed Slack channel", slog.String("channel_id", channelID), slog.String("new_name", name))
	return false, nil
}

func SetChannelDescription(ctx workflow.Context, channelID, title, prURL string) {
	desc := fmt.Sprintf("`%s`", title)
	if len(desc) > channelMetadataMaxLen {
		desc = desc[:channelMetadataMaxLen-4] + "`..."
	}

	if err := slack.ConversationsSetPurpose(ctx, channelID, desc); err != nil {
		logger.From(ctx).Error("failed to set Slack channel description", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("pr_url", prURL))
	}
}

func SetChannelTopic(ctx workflow.Context, channelID, prURL string) {
	topic := prURL
	if len(topic) > channelMetadataMaxLen {
		topic = topic[:channelMetadataMaxLen-4] + " ..."
	}

	if err := slack.ConversationsSetTopic(ctx, channelID, topic); err != nil {
		logger.From(ctx).Error("failed to set Slack channel topic", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("pr_url", prURL))
	}
}
