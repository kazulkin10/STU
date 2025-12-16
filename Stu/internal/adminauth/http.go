package adminauth

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"stu/internal/auth"
)

// RegisterRoutes mounts /v1/admin/auth endpoints.
func RegisterRoutes(r chi.Router, svc *Service, logger zerolog.Logger) {
	r.Post("/login", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Email      string `json:"email"`
			Password   string `json:"password"`
			DeviceName string `json:"device_name"`
			Platform   string `json:"platform"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		token, err := svc.Login(req.Context(), body.Email, body.Password, body.DeviceName, body.Platform, req.UserAgent(), req.RemoteAddr)
		if err != nil {
			if err == ErrNotAdmin {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if err == auth.ErrBanned {
				reason, at := svc.BanInfo()
				writeJSON(w, map[string]any{
					"error":     "banned",
					"reason":    reason,
					"banned_at": at,
				}, http.StatusForbidden)
				return
			}
			if err == ErrSessionExpired {
				http.Error(w, "session expired", http.StatusUnauthorized)
				return
			}
			if err == ErrInvalidCode {
				http.Error(w, "invalid code", http.StatusUnauthorized)
				return
			}
			if err == ErrInvalidStep {
				http.Error(w, "invalid step", http.StatusBadRequest)
				return
			}
			logger.Warn().Err(err).Msg("admin login failed")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, map[string]any{"session_token": token, "next": "totp"}, http.StatusOK)
	})

	r.Post("/totp/init", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			SessionToken string `json:"session_token"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		secret, err := svc.InitTOTP(req.Context(), body.SessionToken)
		if err != nil {
			logger.Warn().Err(err).Msg("totp init failed")
			http.Error(w, "cannot init totp", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"secret": secret}, http.StatusOK)
	})

	r.Post("/totp", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			SessionToken string `json:"session_token"`
			Code         string `json:"code"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		status, err := svc.VerifyTOTP(req.Context(), body.SessionToken, body.Code)
		if err != nil {
			if err == ErrInvalidCode {
				http.Error(w, "invalid code", http.StatusUnauthorized)
				return
			}
			logger.Warn().Err(err).Msg("totp verify failed")
			http.Error(w, "cannot verify", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"status": status, "next": "email_code"}, http.StatusOK)
	})

	r.Post("/email", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			SessionToken string `json:"session_token"`
			Code         string `json:"code"`
			DeviceName   string `json:"device_name"`
			Platform     string `json:"platform"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		tokens, err := svc.VerifyEmail(req.Context(), body.SessionToken, body.Code, body.DeviceName, body.Platform, req.UserAgent(), req.RemoteAddr)
		if err != nil {
			if err == ErrInvalidCode {
				http.Error(w, "invalid code", http.StatusUnauthorized)
				return
			}
			if err == auth.ErrBanned {
				reason, at := svc.BanInfo()
				writeJSON(w, map[string]any{
					"error":     "banned",
					"reason":    reason,
					"banned_at": at,
				}, http.StatusForbidden)
				return
			}
			if err == ErrSessionExpired {
				http.Error(w, "session expired", http.StatusUnauthorized)
				return
			}
			logger.Warn().Err(err).Msg("email verify failed")
			http.Error(w, "cannot verify", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{
			"user_id":       tokens.UserID,
			"device_id":     tokens.DeviceID,
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
		}, http.StatusOK)
	})

	r.Post("/logout", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if err := svc.Logout(req.Context(), body.RefreshToken); err != nil {
			logger.Warn().Err(err).Msg("admin logout failed")
			http.Error(w, "logout failed", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
	})
}

func writeJSON(w http.ResponseWriter, payload any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
