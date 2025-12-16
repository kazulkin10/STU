package reports

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"stu/internal/auth"
)

type createReq struct {
	ReportedUserID string `json:"reported_user_id"`
	DialogID       string `json:"dialog_id"`
	MessageID      *int64 `json:"message_id"`
	Reason         string `json:"reason"`
}

// RegisterUserRoutes mounts user report endpoints.
func RegisterUserRoutes(r chi.Router, svc *Service, logger zerolog.Logger) {
	r.Post("/", func(w http.ResponseWriter, req *http.Request) {
		uid, _, ok := auth.UserFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body createReq
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		repUser, err := uuid.Parse(body.ReportedUserID)
		if err != nil {
			http.Error(w, "invalid reported_user_id", http.StatusBadRequest)
			return
		}
		var dialogID uuid.UUID
		if body.DialogID != "" {
			if dialogID, err = uuid.Parse(body.DialogID); err != nil {
				http.Error(w, "invalid dialog_id", http.StatusBadRequest)
				return
			}
		}
		rep, err := svc.Create(req.Context(), uuid.MustParse(uid), CreateReport{
			ReportedUserID: repUser,
			DialogID:       dialogID,
			MessageID:      body.MessageID,
			Reason:         body.Reason,
		})
		if err != nil {
			logger.Warn().Err(err).Msg("create report failed")
			http.Error(w, "cannot create report", http.StatusBadRequest)
			return
		}
		writeJSON(w, rep, http.StatusCreated)
	})

	r.Get("/mine", func(w http.ResponseWriter, req *http.Request) {
		uid, _, ok := auth.UserFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		reps, err := svc.ListMine(req.Context(), uuid.MustParse(uid))
		if err != nil {
			logger.Error().Err(err).Msg("list reports mine failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, reps, http.StatusOK)
	})
}

// RegisterAdminRoutes mounts admin report endpoints.
func RegisterAdminRoutes(r chi.Router, svc *Service, logger zerolog.Logger) {
	r.Get("/reports", func(w http.ResponseWriter, req *http.Request) {
		status := req.URL.Query().Get("status")
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
		reps, err := svc.ListAdmin(req.Context(), status, limit, offset)
		if err != nil {
			logger.Error().Err(err).Msg("list admin reports failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, reps, http.StatusOK)
	})

	r.Post("/reports/{id}/close", func(w http.ResponseWriter, req *http.Request) {
		reportID, err := uuid.Parse(chi.URLParam(req, "id"))
		if err != nil {
			http.Error(w, "invalid report id", http.StatusBadRequest)
			return
		}
		if err := svc.Close(req.Context(), reportID); err != nil {
			logger.Error().Err(err).Msg("close report failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "closed"}, http.StatusOK)
	})
}

func writeJSON(w http.ResponseWriter, payload any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
