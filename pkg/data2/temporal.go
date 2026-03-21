package data2

import (
	"log/slog"
	"reflect"
	"runtime"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/temporal"
)

var localActivityOpts = workflow.LocalActivityOptions{
	StartToCloseTimeout: 5 * time.Second,
	RetryPolicy: &temporal.RetryPolicy{
		BackoffCoefficient: 1.0,
		MaximumAttempts:    5,
	},
}

// executeLocalActivity runs a Temporal local activity. Use it when you want to execute a
// short-lived, non-deterministic, idempotent function without logging its input or output.
func executeLocalActivity(ctx workflow.Context, activity, result any, args ...any) error {
	fn := runtime.FuncForPC(reflect.ValueOf(activity).Pointer()).Name()
	ctx = workflow.WithLocalActivityOptions(ctx, localActivityOpts)

	start := time.Now()
	err := workflow.ExecuteLocalActivity(ctx, activity, args...).Get(ctx, result)
	logger.From(ctx).Debug("executed local Temporal activity for data access", slog.String("activity", fn),
		slog.Duration("duration", time.Since(start)), slog.Any("error", err))

	return err
}
