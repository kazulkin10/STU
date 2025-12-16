package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"stu/internal/dialogs"
)

// RedisPublisher publishes dialog events into per-user channels.
type RedisPublisher struct {
	rdb *redis.Client
}

func NewRedisPublisher(rdb *redis.Client) *RedisPublisher {
	return &RedisPublisher{rdb: rdb}
}

type event struct {
	Type      string `json:"type"`
	DialogID  string `json:"dialog_id"`
	MessageID int64  `json:"message_id,omitempty"`
	SenderID  string `json:"sender_id,omitempty"`
	Text      string `json:"text,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

func channelForUser(userID uuid.UUID) string {
	return "user:" + userID.String()
}

// PublishMessage sends message.new to members (excluding sender handled by consumer if needed).
func (p *RedisPublisher) PublishMessage(ctx context.Context, msg dialogs.Message, members []uuid.UUID) error {
	payload, _ := json.Marshal(event{
		Type:      "message.new",
		DialogID:  msg.DialogID.String(),
		MessageID: msg.ID,
		SenderID:  msg.SenderID.String(),
		Text:      msg.Text,
		CreatedAt: msg.CreatedAt.UTC().Format(time.RFC3339),
	})
	for _, m := range members {
		if m == msg.SenderID {
			continue // не шлём отправителю
		}
		if err := p.rdb.Publish(ctx, channelForUser(m), payload).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (p *RedisPublisher) PublishDelivery(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64, members []uuid.UUID) error {
	payload, _ := json.Marshal(event{
		Type:      "message.delivered",
		DialogID:  dialogID.String(),
		MessageID: messageID,
		UserID:    userID.String(),
	})
	for _, m := range members {
		if err := p.rdb.Publish(ctx, channelForUser(m), payload).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (p *RedisPublisher) PublishRead(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64, members []uuid.UUID) error {
	payload, _ := json.Marshal(event{
		Type:      "message.read",
		DialogID:  dialogID.String(),
		MessageID: messageID,
		UserID:    userID.String(),
	})
	for _, m := range members {
		if err := p.rdb.Publish(ctx, channelForUser(m), payload).Err(); err != nil {
			return err
		}
	}
	return nil
}
