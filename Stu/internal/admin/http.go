package admin

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"stu/internal/reports"
)

type BanBody struct {
	Reason string `json:"reason"`
}

// RegisterRoutes mounts admin routes under /v1/admin (protected by RequireAdmin).
func RegisterRoutes(r chi.Router, reportsSvc *reports.Service, users UsersService, logger zerolog.Logger) {
	r.Get("/reports", func(w http.ResponseWriter, req *http.Request) {
		status := req.URL.Query().Get("status")
		reps, err := reportsSvc.ListAdmin(req.Context(), status, 50, 0)
		if err != nil {
			logger.Error().Err(err).Msg("admin reports list failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, reps, http.StatusOK)
	})

	r.Post("/users/{id}/ban", func(w http.ResponseWriter, req *http.Request) {
		userID, err := uuid.Parse(chi.URLParam(req, "id"))
		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}
		var body BanBody
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Reason == "" {
			http.Error(w, "reason required", http.StatusBadRequest)
			return
		}
		if err := users.Ban(req.Context(), userID, body.Reason); err != nil {
			logger.Error().Err(err).Msg("ban failed")
			http.Error(w, "ban failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "banned"}, http.StatusOK)
	})

	r.Post("/users/{id}/unban", func(w http.ResponseWriter, req *http.Request) {
		userID, err := uuid.Parse(chi.URLParam(req, "id"))
		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}
		if err := users.Unban(req.Context(), userID); err != nil {
			logger.Error().Err(err).Msg("unban failed")
			http.Error(w, "unban failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "unbanned"}, http.StatusOK)
	})
}

func writeJSON(w http.ResponseWriter, payload any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// UsersService abstracts ban/unban operations.
type UsersService interface {
	Ban(ctx context.Context, userID uuid.UUID, reason string) error
	Unban(ctx context.Context, userID uuid.UUID) error
}
