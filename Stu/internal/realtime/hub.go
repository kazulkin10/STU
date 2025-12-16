package realtime

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"stu/internal/auth"
)

// AccessValidator defines minimal interface for access token validation.
type AccessValidator interface {
	ValidateAccessToken(ctx context.Context, hash []byte) (auth.SessionInfo, error)
}

type Hub struct {
	logger    zerolog.Logger
	upgrader  websocket.Upgrader
	rdb       *redis.Client
	validator AccessValidator
	connsMu   sync.RWMutex
	conns     map[string]*websocket.Conn // userID -> conn (single per user for simplicity)
}

func NewHub(logger zerolog.Logger, rdb *redis.Client, validator AccessValidator) *Hub {
	return &Hub{
		logger:    logger,
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		rdb:       rdb,
		validator: validator,
		conns:     make(map[string]*websocket.Conn),
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	authz := r.Header.Get("Authorization")
	var token string
	if strings.HasPrefix(authz, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authz, "Bearer"))
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	hash := sha256.Sum256([]byte(token))
	session, err := h.validator.ValidateAccessToken(r.Context(), hash[:])
	if err != nil {
		if err == auth.ErrBanned {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"banned"}`))
			return
		}
		h.logger.Warn().Err(err).Msg("ws auth failed")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn().Err(err).Msg("ws upgrade failed")
		return
	}
	userID := session.UserID
	h.storeConn(userID, conn)
	go h.subscribeUser(context.Background(), userID)

	// basic ping/pong loop
	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := conn.NextReader(); err != nil {
			break
		}
	}
	h.removeConn(userID)
}

func (h *Hub) subscribeUser(ctx context.Context, userID string) {
	channel := userChannel(userID)
	sub := h.rdb.Subscribe(ctx, channel)
	defer sub.Close()
	ch := sub.Channel()
	for msg := range ch {
		h.sendTo(userID, []byte(msg.Payload))
	}
}

func userChannel(userID string) string {
	return "user:" + userID
}

func (h *Hub) sendTo(userID string, payload []byte) {
	h.connsMu.RLock()
	conn, ok := h.conns[userID]
	h.connsMu.RUnlock()
	if !ok {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}

func (h *Hub) storeConn(userID string, conn *websocket.Conn) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	// close existing
	if old, ok := h.conns[userID]; ok {
		_ = old.Close()
	}
	h.conns[userID] = conn
}

func (h *Hub) removeConn(userID string) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	delete(h.conns, userID)
}
