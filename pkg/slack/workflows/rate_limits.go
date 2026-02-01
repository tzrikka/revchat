package workflows

import (
	"log/slog"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// AppRateLimitedWorkflow highlights Slack rate-limiting events:
//   - https://docs.slack.dev/apis/events-api/#rate-limiting
//   - https://docs.slack.dev/reference/events/app_rate_limited
func (c *Config) AppRateLimitedWorkflow(ctx workflow.Context, event map[string]any) error {
	logger.From(ctx).Error("Slack app is rate limited", slog.Any("event", event))
	activities.AlertWarn(ctx, c.AlertsChannel, "Slack app is rate limited!")

	// For extra visibility, even though this isn't strictly a workflow error.
	return temporal.NewNonRetryableApplicationError("Slack app is rate limited", "slack.events.app_rate_limited", nil, event)
}
