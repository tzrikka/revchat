package activities

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

const (
	channelMetadataMaxLen = 250
)

// LookupChannel returns the ID of a Slack channel associated with the given PR, if it exists.
func LookupChannel(ctx workflow.Context, prURL string) (string, bool) {
	if prURL == "" {
		return "", false
	}

	channelID, _ := data.SwitchURLAndID(ctx, prURL)
	return channelID, channelID != ""
}

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

// InviteUsersToChannel adds up to 1,000 users to the given Slack channel
// and PR attention state (the given users are expected to be opted-in).
// This is an idempotent function, unlike the underlying Slack API call.
func InviteUsersToChannel(ctx workflow.Context, channelID, prURL string, participantIDs, followerIDs []string) error {
	// API limitation, but we don't split into multiple API calls because that many reviewers is undesirable anyway.
	if len(participantIDs) > 1000 {
		logger.From(ctx).Warn("trying to add more than 1000 users to Slack channel - truncating",
			slog.String("channel_id", channelID), slog.Int("users_len", len(participantIDs)))
		participantIDs = participantIDs[:1000]
	}

	var errs []error
	var dontInvite []string
	for _, id := range participantIDs {
		if id == "" {
			dontInvite = append(dontInvite, id)
			continue
		}
		if err := data.AddReviewerToTurns(ctx, prURL, users.SlackIDToEmail(ctx, id)); err != nil {
			dontInvite = append(dontInvite, id)
			errs = append(errs, err)
		}
	}

	// Don't invite users we failed to add to the PR's attention state, for consistency.
	for _, id := range dontInvite {
		i := slices.Index(participantIDs, id)
		participantIDs = slices.Delete(participantIDs, i, i+1)
	}
	if len(participantIDs) == 0 {
		return errors.Join(errs...)
	}

	// But do invite followers of the PR author, without checking opt-in status (can't
	// follow without being opted-in) and without adding them to the PR's attention state.
	userIDs := slices.Concat(participantIDs, followerIDs)
	slices.Sort(userIDs)
	userIDs = slices.Compact(userIDs)

	if err := slack.ConversationsInvite(ctx, channelID, userIDs, true); err != nil {
		msg := "failed to add user(s) to Slack channel"

		if strings.Contains(err.Error(), "already_in_channel") {
			msg += " - already in channel" // This is not a problem.
			logger.From(ctx).Debug(msg, slog.Any("error", err), slog.String("channel_id", channelID),
				slog.String("user_ids", strings.Join(userIDs, ",")))
			return errors.Join(errs...)
		}

		logger.From(ctx).Error(msg, slog.Any("error", err), slog.String("channel_id", channelID),
			slog.String("user_ids", strings.Join(userIDs, ",")))
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// KickUsersFromChannel removes the given users from the given Slack channel and PR
// attention state. This is an idempotent function, unlike the underlying Slack API call.
func KickUsersFromChannel(ctx workflow.Context, channelID, prURL string, userIDs []string) error {
	var errs []error
	for _, id := range userIDs {
		if id == "" {
			continue
		}

		err := slack.ConversationsKick(ctx, channelID, id)
		if err != nil {
			msg := "failed to remove user from Slack channel"

			if strings.Contains(err.Error(), "not_in_channel") {
				msg += " - not in channel" // This is not a problem.
				logger.From(ctx).Debug(msg, slog.Any("error", err),
					slog.String("channel_id", channelID), slog.String("user_id", id))
			} else {
				logger.From(ctx).Error(msg, slog.Any("error", err),
					slog.String("channel_id", channelID), slog.String("user_id", id))
				errs = append(errs, err)
				continue
			}
		}

		if err := data.RemoveReviewerFromTurns(ctx, prURL, users.SlackIDToEmail(ctx, id), false); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
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

func SetChannelDescription(ctx workflow.Context, channelID, title, prURL, email string) {
	data.UpdateActivityTime(ctx, prURL, email)

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
