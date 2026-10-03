package service

import "context"

type recoveryContextKey struct{}

// WithRecoveryContext marks an application call as durable migration recovery.
// The write use case remains identical to the gRPC path, while non-critical
// projections may be dispatched asynchronously after the database write.
func WithRecoveryContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, recoveryContextKey{}, true)
}

func isRecoveryContext(ctx context.Context) bool {
	value, _ := ctx.Value(recoveryContextKey{}).(bool)
	return value
}
