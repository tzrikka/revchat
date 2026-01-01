package bitbucket

import (
	"errors"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// LookupSlackChannel returns the ID of a Slack channel associated with the given PR, if it exists.
func LookupSlackChannel(ctx workflow.Context, eventType, prURL string) (string, bool) {
	if prURL == "" {
		return "", false
	}

	channelID, err := data.SwitchURLAndID(ctx, prURL)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve PR's Slack channel ID", slog.Any("error", err),
			slog.String("event_type", eventType), slog.String("pr_url", prURL))
		return "", false
	}

	return channelID, channelID != ""
}

func CreateSlackChannel(ctx workflow.Context, pr PullRequest, maxLen int, prefix string, private bool) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, maxLen)
	prURL := HTMLURL(pr.Links)

	for i := 1; i < 10; i++ {
		name := fmt.Sprintf("%s-%d_%s", prefix, pr.ID, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		id, retry, err := activities.CreateChannel(ctx, name, prURL, private)
		if err != nil {
			if retry {
				continue
			} else {
				return "", err
			}
		}

		if err := data.LogSlackChannelCreated(id, name, prURL); err != nil {
			logger.From(ctx).Error("failed to log Slack channel creation", slog.Any("error", err),
				slog.String("channel_id", id), slog.String("pr_url", prURL))
			// Don't return the error (i.e. don't abort the calling workflow because of logging errors).
		}

		return id, nil
	}

	logger.From(ctx).Error("too many failed attempts to create Slack channel", slog.String("pr_url", prURL))
	return "", errors.New("too many failed attempts to create Slack channel")
}

func RenameSlackChannel(ctx workflow.Context, pr PullRequest, channelID string, maxLen int, prefix string) error {
	title := slack.NormalizeChannelName(pr.Title, maxLen)

	for i := 1; i < 10; i++ {
		name := fmt.Sprintf("%s-%d_%s", prefix, pr.ID, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		retry, err := activities.RenameChannel(ctx, channelID, name)
		if retry {
			continue
		}

		if err == nil {
			if logErr := data.LogSlackChannelRenamed(channelID, name); logErr != nil {
				logger.From(ctx).Error("failed to log Slack channel renaming", slog.Any("error", logErr),
					slog.String("channel_id", channelID), slog.String("new_name", name))
				// Don't return the error (i.e. don't abort the calling workflow because of logging errors).
			}
		}

		return err
	}

	logger.From(ctx).Error("too many failed attempts to rename Slack channel",
		slog.String("pr_url", HTMLURL(pr.Links)), slog.String("channel_id", channelID))
	return errors.New("too many failed attempts to rename Slack channel")
}
