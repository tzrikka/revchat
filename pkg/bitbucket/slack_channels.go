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

	channelID, _ := data.SwitchURLAndID(ctx, prURL)
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

		data.LogSlackChannelCreated(ctx, id, name, prURL)
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
			data.LogSlackChannelRenamed(ctx, channelID, name)
		}
		return err
	}

	logger.From(ctx).Error("too many failed attempts to rename Slack channel",
		slog.String("pr_url", HTMLURL(pr.Links)), slog.String("channel_id", channelID))
	return errors.New("too many failed attempts to rename Slack channel")
}
