package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse describes health payload.
type HealthResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthHandler returns OK response with metadata.
func HealthHandler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := HealthResponse{
			Status:    "ok",
			Service:   service,
			Timestamp: time.Now().UTC(),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
