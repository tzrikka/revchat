// Package otel provides lightweight wrapper functions to
// record OpenTelemetry metrics using Temporal activities.
package otel

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

const (
	name = "github.com/tzrikka/revchat/internal/otel"
)

var (
	disabled = false

	activityOpts = workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: time.Second,
	}
)

type activityRequest struct {
	Name  string
	Inc   int64
	Attrs map[string]string
}

// InitMetrics initializes OpenTelemetry metrics exporting using OTLP over HTTP.
func InitMetrics(ctx context.Context, cmd *cli.Command) (*metric.MeterProvider, error) {
	disabled = cmd.Bool("otlp-disabled")
	if disabled {
		return nil, nil
	}

	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpointURL(cmd.String("otlp-endpoint")),
		otlpmetrichttp.WithTimeout(time.Duration(cmd.Int("otlp-timeout-ms")) * time.Millisecond),
	}
	switch compression := strings.ToLower(cmd.String("otlp-compression")); compression {
	case "gzip":
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression))
	case "", "none":
		// Do nothing.
	default:
		return nil, errors.New("unrecognized OTLP compression method: " + compression)
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
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
	if disabled {
		return
	}

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
