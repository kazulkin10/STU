package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// RegisterHandlers mounts auth HTTP routes under /v1/auth.
func RegisterHandlers(r chi.Router, svc *Service, logger zerolog.Logger) {
	r.Route("/auth", func(api chi.Router) {
		api.Post("/register", func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			payload.Email = strings.TrimSpace(payload.Email)
			if payload.Email == "" || payload.Password == "" {
				http.Error(w, "email and password required", http.StatusBadRequest)
				return
			}
			userID, err := svc.Register(req.Context(), payload.Email, payload.Password)
			if err != nil {
				if err == ErrUserExists {
					http.Error(w, "user already exists", http.StatusConflict)
					return
				}
				logger.Error().Err(err).Msg("register failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			resp := map[string]any{
				"user_id": userID.String(),
				"status":  "verification_sent",
			}
			writeJSON(w, resp, http.StatusCreated)
		})

		api.Post("/verify", func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				Email      string `json:"email"`
				Code       string `json:"code"`
				DeviceName string `json:"device_name"`
				Platform   string `json:"platform"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if payload.Email == "" || payload.Code == "" {
				http.Error(w, "email and code required", http.StatusBadRequest)
				return
			}
			ip := remoteIP(req)
			userID, deviceID, access, refresh, err := svc.Verify(req.Context(), payload.Email, payload.Code, payload.DeviceName, payload.Platform, req.UserAgent(), ip)
			if err != nil {
				if err == ErrBanned {
					reason, at := svc.LastBanInfo()
					writeJSON(w, map[string]any{
						"error":     "banned",
						"reason":    reason,
						"banned_at": at,
					}, http.StatusForbidden)
					return
				}
				logger.Warn().Err(err).Msg("verify failed")
				http.Error(w, "invalid code", http.StatusUnauthorized)
				return
			}
			resp := map[string]any{
				"user_id":       userID.String(),
				"device_id":     deviceID.String(),
				"access_token":  access,
				"refresh_token": refresh,
			}
			writeJSON(w, resp, http.StatusOK)
		})

		api.Post("/login", func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				Email      string `json:"email"`
				Password   string `json:"password"`
				DeviceName string `json:"device_name"`
				Platform   string `json:"platform"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			ip := remoteIP(req)
			userID, deviceID, access, refresh, err := svc.Login(req.Context(), payload.Email, payload.Password, payload.DeviceName, payload.Platform, req.UserAgent(), ip)
			if err != nil {
				logger.Warn().Err(err).Msg("login failed")
				if err == ErrBanned {
					reason, at := svc.LastBanInfo()
					writeJSON(w, map[string]any{
						"error":     "banned",
						"reason":    reason,
						"banned_at": at,
					}, http.StatusForbidden)
					return
				}
				if err == ErrInactive {
					http.Error(w, "account not verified", http.StatusForbidden)
					return
				}
				http.Error(w, "invalid credentials", http.StatusUnauthorized)
				return
			}
			resp := map[string]any{
				"user_id":       userID.String(),
				"device_id":     deviceID.String(),
				"access_token":  access,
				"refresh_token": refresh,
			}
			writeJSON(w, resp, http.StatusOK)
		})

		api.Post("/refresh", func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			access, refresh, err := svc.Refresh(req.Context(), payload.RefreshToken, req.UserAgent(), req.RemoteAddr)
			if err != nil {
				logger.Warn().Err(err).Msg("refresh failed")
				if err == ErrRefreshReuse {
					http.Error(w, "refresh reuse detected", http.StatusUnauthorized)
					return
				}
				http.Error(w, "invalid refresh token", http.StatusUnauthorized)
				return
			}
			resp := map[string]any{
				"access_token":  access,
				"refresh_token": refresh,
			}
			writeJSON(w, resp, http.StatusOK)
		})

		api.Post("/logout", func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if err := svc.Logout(req.Context(), payload.RefreshToken); err != nil {
				logger.Warn().Err(err).Msg("logout failed")
				http.Error(w, "invalid refresh token", http.StatusUnauthorized)
				return
			}
			writeJSON(w, map[string]string{"status": "revoked"}, http.StatusOK)
		})

		api.Post("/logout_all", func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if err := svc.LogoutAll(req.Context(), payload.RefreshToken); err != nil {
				logger.Warn().Err(err).Msg("logout all failed")
				http.Error(w, "invalid refresh token", http.StatusUnauthorized)
				return
			}
			writeJSON(w, map[string]string{"status": "revoked_all"}, http.StatusOK)
		})
	})
}

func writeJSON(w http.ResponseWriter, payload any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func remoteIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		candidate := strings.TrimSpace(parts[0])
		if candidate != "" {
			return candidate
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
