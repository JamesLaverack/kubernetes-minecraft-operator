package logutil

import (
	"context"

	"go.uber.org/zap"
)

const LoggerKey = "ZapLogger"

// FromContextOrNew will either pull an existing *zap.Logger from the context, or create a new one. It doesn't then add
// that new logger to the context, that's left as a task for the caller.
func FromContextOrNew(ctx context.Context) *zap.Logger {
	l := ctx.Value(LoggerKey)
	log, ok := l.(*zap.Logger)
	if !ok {
		log, err := zap.NewProduction()
		if err != nil {
			panic(err)
		}
		return log
	}
	return log
}

// IntoContext returns a new context based on ctx with the provided *zap.Logger added. This can be later retrieved using
// the FromContextOrNew function.
func IntoContext(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, log)
}
