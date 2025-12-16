package dialogs

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"stu/internal/auth"
)

type createDialogRequest struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

type sendMessageRequest struct {
	Text string `json:"text"`
}

// RegisterHandlers mounts dialog routes under /v1/dialogs.
func RegisterHandlers(r chi.Router, svc *Service, logger zerolog.Logger) {
	r.Post("/", func(w http.ResponseWriter, req *http.Request) {
		curUser, _, ok := auth.UserFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if auth.IsBanned(req.Context()) {
			http.Error(w, `{"error":"banned"}`, http.StatusForbidden)
			return
		}
		var payload createDialogRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		target := payload.UserID
		if target == "" {
			target = payload.Email
		}
		if target == "" {
			http.Error(w, "user_id or email required", http.StatusBadRequest)
			return
		}
		dialogID, err := svc.CreateDirect(req.Context(), uuid.MustParse(curUser), target)
		if err != nil {
			logger.Warn().Err(err).Msg("create dialog failed")
			http.Error(w, "cannot create dialog", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"dialog_id": dialogID.String()}, http.StatusCreated)
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		curUser, _, ok := auth.UserFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		dialogs, err := svc.ListDialogs(req.Context(), uuid.MustParse(curUser), 50)
		if err != nil {
			logger.Error().Err(err).Msg("list dialogs failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, dialogs, http.StatusOK)
	})

	r.Route("/{id}", func(rt chi.Router) {
		rt.Get("/messages", func(w http.ResponseWriter, req *http.Request) {
			curUser, _, ok := auth.UserFromContext(req.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if auth.IsBanned(req.Context()) {
				http.Error(w, `{"error":"banned"}`, http.StatusForbidden)
				return
			}
			dialogID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				http.Error(w, "invalid dialog id", http.StatusBadRequest)
				return
			}
			limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
			before, _ := strconv.ParseInt(req.URL.Query().Get("before"), 10, 64)
			msgs, err := svc.ListMessages(req.Context(), uuid.MustParse(curUser), dialogID, limit, before)
			if err != nil {
				if err == ErrForbidden {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				logger.Error().Err(err).Msg("list messages failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, msgs, http.StatusOK)
		})

		rt.Post("/messages", func(w http.ResponseWriter, req *http.Request) {
			curUser, _, ok := auth.UserFromContext(req.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if auth.IsBanned(req.Context()) {
				http.Error(w, `{"error":"banned"}`, http.StatusForbidden)
				return
			}
			dialogID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				http.Error(w, "invalid dialog id", http.StatusBadRequest)
				return
			}
			var payload sendMessageRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if payload.Text == "" {
				http.Error(w, "text required", http.StatusBadRequest)
				return
			}
			msg, err := svc.SendMessage(req.Context(), uuid.MustParse(curUser), dialogID, payload.Text)
			if err != nil {
				if err == ErrForbidden {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				logger.Error().Err(err).Msg("send message failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, msg, http.StatusCreated)
		})

		rt.Post("/messages/{mid}/delivered", func(w http.ResponseWriter, req *http.Request) {
			curUser, _, ok := auth.UserFromContext(req.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if auth.IsBanned(req.Context()) {
				http.Error(w, `{"error":"banned"}`, http.StatusForbidden)
				return
			}
			dialogID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				http.Error(w, "invalid dialog id", http.StatusBadRequest)
				return
			}
			mid, err := strconv.ParseInt(chi.URLParam(req, "mid"), 10, 64)
			if err != nil {
				http.Error(w, "invalid message id", http.StatusBadRequest)
				return
			}
			if err := svc.MarkDelivered(req.Context(), uuid.MustParse(curUser), dialogID, mid); err != nil {
				if err == ErrForbidden {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				http.Error(w, "cannot mark delivered", http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
		})

		rt.Post("/messages/{mid}/read", func(w http.ResponseWriter, req *http.Request) {
			curUser, _, ok := auth.UserFromContext(req.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if auth.IsBanned(req.Context()) {
				http.Error(w, `{"error":"banned"}`, http.StatusForbidden)
				return
			}
			dialogID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				http.Error(w, "invalid dialog id", http.StatusBadRequest)
				return
			}
			mid, err := strconv.ParseInt(chi.URLParam(req, "mid"), 10, 64)
			if err != nil {
				http.Error(w, "invalid message id", http.StatusBadRequest)
				return
			}
			if err := svc.MarkRead(req.Context(), uuid.MustParse(curUser), dialogID, mid); err != nil {
				if err == ErrForbidden {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				http.Error(w, "cannot mark read", http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
		})
	})
}

func writeJSON(w http.ResponseWriter, payload any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
