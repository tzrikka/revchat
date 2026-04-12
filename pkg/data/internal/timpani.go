package internal

import (
	"context"
	"fmt"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/tzrikka/timpani-api/pkg/temporal"
)

// executeTimpaniActivity starts a Timpani activity via a Temporal client instead of calling the Timpani API.
// The Timpani API is meant for Temporal workflows, but this package uses nondeterministic Temporal activities.
func executeTimpaniActivity(ctx context.Context, opts client.Options, activity, userID string, req, result any) error {
	c, err := client.Dial(opts)
	if err != nil {
		return err
	}
	defer c.Close()

	startOpts := client.StartActivityOptions{
		ID:                       fmt.Sprintf("%s_%s", activity, userID),
		ActivityIDConflictPolicy: enums.ACTIVITY_ID_CONFLICT_POLICY_USE_EXISTING,
		ActivityIDReusePolicy:    enums.ACTIVITY_ID_REUSE_POLICY_ALLOW_DUPLICATE,

		TaskQueue:              temporal.ActivityOptions.TaskQueue,
		ScheduleToCloseTimeout: temporal.ActivityOptions.ScheduleToCloseTimeout,
		ScheduleToStartTimeout: temporal.ActivityOptions.ScheduleToStartTimeout,
		StartToCloseTimeout:    temporal.ActivityOptions.StartToCloseTimeout,
		HeartbeatTimeout:       temporal.ActivityOptions.HeartbeatTimeout,
		RetryPolicy:            temporal.ActivityOptions.RetryPolicy,
	}

	handle, err := c.ExecuteActivity(ctx, startOpts, activity, req)
	if err != nil {
		return err
	}

	return handle.Get(ctx, result)
}
