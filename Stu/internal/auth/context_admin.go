package auth

import "context"

type adminKey struct{}

// WithAdmin sets admin flag in context.
func WithAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, adminKey{}, isAdmin)
}

// IsAdmin returns admin flag.
func IsAdmin(ctx context.Context) bool {
	val, ok := ctx.Value(adminKey{}).(bool)
	return ok && val
}
