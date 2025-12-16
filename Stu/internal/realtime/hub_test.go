package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"stu/internal/auth"
)

type stubAuthRepo struct {
	userID   uuid.UUID
	deviceID uuid.UUID
}

func (s *stubAuthRepo) ValidateAccessToken(ctx context.Context, accessHash []byte) (auth.SessionInfo, error) {
	return auth.SessionInfo{
		UserID:   s.userID.String(),
		DeviceID: s.deviceID.String(),
	}, nil
}

func TestHubReceivesEvent(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	logger := zerolog.New(zerolog.NewTestWriter(t))
	repo := &stubAuthRepo{userID: uuid.New(), deviceID: uuid.New()}
	hub := NewHub(logger, rdb, repo)

	srv := httptest.NewServer(hubHandler(hub))
	defer srv.Close()

	u := "ws" + srv.URL[len("http"):] + "/v1/ws"

	dialer := websocket.Dialer{}
	token := "abc"
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+token)
	conn, _, err := dialer.Dial(u, header)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)
	// publish event to user channel
	payload := `{"type":"message.new","dialog_id":"d","message_id":1}`
	if err := rdb.Publish(context.Background(), "user:"+repo.userID.String(), payload).Err(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if string(msg) != payload {
		t.Fatalf("unexpected payload: %s", msg)
	}
}

// helper to wrap hub.HandleWS into http.Handler
func hubHandler(h *Hub) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/ws", h.HandleWS)
	return mux
}
