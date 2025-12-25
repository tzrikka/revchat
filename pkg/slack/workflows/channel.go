package workflows

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
)

// ChannelArchivedWorkflow handles PR data cleanups after unexpected Slack archiving events,
// so RevChat will not try to modify or post messages to archived channels:
//   - https://docs.slack.dev/reference/events/channel_archive/
//   - https://docs.slack.dev/reference/events/group_archive/
func ChannelArchivedWorkflow(ctx workflow.Context, event archiveEventWrapper) error {
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

	data.CleanupPRData(ctx, channelID, url)
	return nil
}
