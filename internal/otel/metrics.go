// Package otel provides lightweight wrapper functions to
// record OpenTelemetry metrics using Temporal activities.
package otel

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

const name = "github.com/tzrikka/revchat/internal/otel"

var activityOpts = workflow.LocalActivityOptions{
	ScheduleToCloseTimeout: time.Second,
}

type activityRequest struct {
	Name  string
	Inc   int64
	Attrs map[string]string
}

func InitMetrics() (*metric.MeterProvider, error) {
	exporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	reader := metric.NewPeriodicReader(exporter)
	res := resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("revchat"))
	provider := metric.NewMeterProvider(metric.WithReader(reader), metric.WithResource(res))

	otel.SetMeterProvider(provider)
	return provider, nil
}

// IncrementCounter increments a metric counter. Attributes are optional.
func IncrementCounter(ctx workflow.Context, name string, incr int64, attrs map[string]string) {
	req := activityRequest{Name: name, Inc: incr, Attrs: attrs}
	ctx = workflow.WithLocalActivityOptions(ctx, activityOpts)
	if err := workflow.ExecuteLocalActivity(ctx, incrementCounterActivity, req).Get(ctx, nil); err != nil {
		logger.From(ctx).Error("failed to increment metric counter", slog.Any("error", err),
			slog.String("name", name), slog.Any("attrs", attrs))
	}
}

func incrementCounterActivity(ctx context.Context, req activityRequest) error {
	meter := otel.GetMeterProvider().Meter(name)
	counter, err := meter.Int64Counter(req.Name)
	if err != nil {
		return err
	}

	attrs := make([]attribute.KeyValue, 0, len(req.Attrs))
	for k, v := range req.Attrs {
		if v == "" {
			continue
		}
		attrs = append(attrs, attribute.String(k, v))
	}

	counter.Add(ctx, req.Inc, otelmetric.WithAttributes(attrs...))
	return nil
}

// SignalReceived increments a metric that a Temporal signal was
// received from Timpani, triggered by an incoming webhook event.
func SignalReceived(ctx workflow.Context, name string, draining bool) {
	msg := "received signal"
	if draining {
		msg += " while draining"
	}
	logger.From(ctx).Info(msg, slog.String("signal_name", name))
	IncrementCounter(ctx, "signal.received", 1, map[string]string{"signal_name": name})
}
