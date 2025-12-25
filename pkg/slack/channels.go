package slack

import (
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
)

// channelArchivedWorkflow handles PR data cleanup after unexpected Slack archiving events:
//   - https://docs.slack.dev/reference/events/channel_archive/
//   - https://docs.slack.dev/reference/events/group_archive/
func (c *Config) channelArchivedWorkflow(ctx workflow.Context, event archiveEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	// Channel archived by someone other than RevChat. The most common reason is
	// that the last member has left the channel, so Slackbot auto-archived it.
	channelID := event.InnerEvent.Channel
	logger.From(ctx).Info("Slack channel archived by someone else",
		slog.String("channel_id", channelID), slog.String("user", event.InnerEvent.User))

	url, err := data.SwitchURLAndID(channelID)
	if err != nil {
		logger.From(ctx).Error("failed to convert Slack channel to PR URL",
			slog.Any("error", err), slog.String("channel_id", channelID))
		return err
	}

	data.FullPRCleanup(ctx, channelID, url)
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
