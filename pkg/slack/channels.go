package slack

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// CreateChannel creates a Slack channel for a new pull/merge request, and returns the channel ID.
// It is a lightweight wrapper for [activities.CreateChannel] which utilizes [NormalizeChannelName].
// The first 3 parameters describe the PR, and the last 3 parameters are RevChat configuration settings.
func CreateChannel(ctx workflow.Context, prID int, prTitle, prURL string, maxLen int, prefix string, private bool) (string, error) {
	title := NormalizeChannelName(prTitle, maxLen)

	for i := 1; i < 20; i++ {
		name := fmt.Sprintf("%s-%d_%s", prefix, prID, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		id, retry, err := activities.CreateChannel(ctx, name, prURL, private)
		if err != nil {
			if retry {
				continue
			}
			return "", err
		}

		data.LogSlackChannelCreated(ctx, id, name, prURL)
		return id, nil
	}

	logger.From(ctx).Error("too many failed attempts to create Slack channel", slog.String("pr_url", prURL))
	return "", errors.New("too many failed attempts to create Slack channel")
}

// RenameChannel renames an existing Slack channel when the title of its corresponding pull/merge request
// changes. It is a lightweight wrapper for [activities.RenameChannel] which utilizes [NormalizeChannelName].
// The first 3 parameters describe the PR, and the last 2 parameters are RevChat configuration settings.
func RenameChannel(ctx workflow.Context, prID int, prTitle, prURL, channelID string, maxLen int, prefix string) error {
	title := NormalizeChannelName(prTitle, maxLen)

	for i := 1; i < 20; i++ {
		name := fmt.Sprintf("%s-%d_%s", prefix, prID, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		if retry, err := activities.RenameChannel(ctx, channelID, name); !retry {
			return err
		}
	}

	logger.From(ctx).Error("too many failed attempts to rename Slack channel",
		slog.String("pr_url", prURL), slog.String("channel_id", channelID))
	return errors.New("too many failed attempts to rename Slack channel")
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
