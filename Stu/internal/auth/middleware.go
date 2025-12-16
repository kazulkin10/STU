package auth

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// AccessValidator validates access tokens.
type AccessValidator interface {
	ValidateAccessToken(ctx context.Context, hash []byte) (SessionInfo, error)
}

// AuthMiddleware checks Bearer token and sets user/device in context.
func AuthMiddleware(logger zerolog.Logger, validator AccessValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer"))
			if token == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}
			hash := sha256.Sum256([]byte(token))
			session, err := validator.ValidateAccessToken(r.Context(), hash[:])
			if err != nil {
				if err == ErrBanned {
					writeJSON(w, map[string]any{
						"error":     "banned",
						"reason":    session.BanReason,
						"banned_at": session.BanAt,
					}, http.StatusForbidden)
					return
				}
				logger.Warn().Err(err).Msg("access token invalid")
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := WithUser(r.Context(), session.UserID, session.DeviceID)
			ctx = WithBan(ctx, session.Banned)
			if session.IsAdmin {
				ctx = WithAdmin(ctx, true)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
