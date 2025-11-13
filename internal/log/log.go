package log

import (
	"go.temporal.io/sdk/workflow"
)

func Debug(ctx workflow.Context, msg string, keyvals ...any) {
	if ctx == nil {
		return
	}
	workflow.GetLogger(ctx).Debug(msg, keyvals...)
}

func Info(ctx workflow.Context, msg string, keyvals ...any) {
	if ctx == nil {
		return
	}
	workflow.GetLogger(ctx).Info(msg, keyvals...)
}

func Warn(ctx workflow.Context, msg string, keyvals ...any) {
	if ctx == nil {
		return
	}
	workflow.GetLogger(ctx).Warn(msg, keyvals...)
}

func Error(ctx workflow.Context, msg string, keyvals ...any) {
	if ctx == nil {
		return
	}
	workflow.GetLogger(ctx).Error(msg, keyvals...)
}
