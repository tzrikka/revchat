// Package logger provides utilities for working with [slog] and [context.Context],
// as well as a Temporal [log.Logger] wrapper for them.
package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

type ctxKey struct{}

var ctxLoggerKey = ctxKey{}

// WithContext returns a derived [context.Context] that points to
// the given parent, and has the given [slog.Logger] attached to it.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey, l)
}

// FromContext returns the [slog.Logger] attached to the given
// [context.Context], or [slog.Default] if none is attached.
func FromContext(ctx context.Context) *slog.Logger {
	l := slog.Default()
	if ctxLogger, ok := ctx.Value(ctxLoggerKey).(*slog.Logger); ok {
		l = ctxLogger
	}
	return l
}

func Fatal(msg string, err error, attrs ...slog.Attr) {
	fatalErrorCtx(context.Background(), msg, err, attrs...)
}

func FatalContext(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	fatalErrorCtx(ctx, msg, err, attrs...)
}

func fatalErrorCtx(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // Discard wrapper frames (Callers, fatalErrorCtx, Fatal*).

	r := slog.NewRecord(time.Now(), slog.LevelError, msg, pcs[0])
	if err != nil {
		r.AddAttrs(slog.Any("error", err))
	}
	r.AddAttrs(attrs...)

	_ = slog.Default().Handler().Handle(ctx, r)
	os.Exit(1)
}

// From returns a Temporal [log.Logger] that wraps a [slog.Logger] which is attached
// to the specified Temporal [workflow.Context]. If the context is nil, it returns
// [slog.Default] as a fallback, which still satisfies the [log.Logger] interface.
func From(ctx workflow.Context) log.Logger {
	if ctx == nil {
		return slog.Default()
	}
	return workflow.GetLogger(ctx)
}
