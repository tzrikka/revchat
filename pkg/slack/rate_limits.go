package slack

import (
	"log/slog"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

// https://docs.slack.dev/apis/events-api/#rate-limiting
// https://docs.slack.dev/reference/events/app_rate_limited
func (c *Config) appRateLimitedWorkflow(ctx workflow.Context, event map[string]any) error {
	msg := "Slack app is rate limited"
	logger.From(ctx).Error(msg, slog.Any("event", event))
	// For extra visibility, even though this isn't strictly a workflow error.
	return temporal.NewNonRetryableApplicationError(msg, "slack.events.app_rate_limited", nil, event)
}
