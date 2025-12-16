package auth

import "context"

type banKey struct{}

// WithBan sets banned flag in context.
func WithBan(ctx context.Context, banned bool) context.Context {
	return context.WithValue(ctx, banKey{}, banned)
}

// IsBanned returns ban flag.
func IsBanned(ctx context.Context) bool {
	val, ok := ctx.Value(banKey{}).(bool)
	return ok && val
}
