package auth

import "context"

type contextKey string

const (
	userIDKey   contextKey = "user_id"
	deviceIDKey contextKey = "device_id"
)

// WithUser stores user and device IDs in context.
func WithUser(ctx context.Context, userID, deviceID string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, deviceIDKey, deviceID)
}

// UserFromContext returns user and device IDs if present.
func UserFromContext(ctx context.Context) (userID string, deviceID string, ok bool) {
	uid, uok := ctx.Value(userIDKey).(string)
	did, dok := ctx.Value(deviceIDKey).(string)
	if !uok || !dok {
		return "", "", false
	}
	return uid, did, true
}
