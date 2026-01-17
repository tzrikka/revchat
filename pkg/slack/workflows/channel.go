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

	// Instead of calling ![isRevChatChannel], because we also need the PR's URL below.
	prURL, _ := data.SwitchURLAndID(ctx, event.InnerEvent.Channel)
	if prURL == "" {
		return nil
	}

	// RevChat channel archived by someone other than RevChat. The most common reason
	// is that the last member has left the channel, so Slackbot auto-archived it.
	logger.From(ctx).Info("RevChat channel archived by someone else",
		slog.String("channel_id", event.InnerEvent.Channel), slog.String("user", event.InnerEvent.User))

	data.CleanupPRData(ctx, event.InnerEvent.Channel, prURL)
	return nil
}
