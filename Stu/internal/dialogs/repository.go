package dialogs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotMember      = errors.New("not a dialog member")
	ErrDialogExist    = errors.New("dialog already exists")
	ErrDialogNotFound = errors.New("dialog not found")
)

type Message struct {
	ID            int64     `json:"id"`
	SenderID      uuid.UUID `json:"sender_id"`
	DialogID      uuid.UUID `json:"dialog_id"`
	Text          string    `json:"text"`
	CreatedAt     time.Time `json:"created_at"`
	DeliveredToMe bool      `json:"delivered_to_me"`
	ReadByMe      bool      `json:"read_by_me"`
	DeliveredPeer bool      `json:"delivered_by_peer"`
	ReadPeer      bool      `json:"read_by_peer"`
}

type Dialog struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	LastMessage *Message  `json:"last_message,omitempty"`
	UnreadCount int64     `json:"unread_count"`
}

type Repository interface {
	CreateDirect(ctx context.Context, initiator uuid.UUID, peer uuid.UUID) (uuid.UUID, error)
	GetOrCreateDirect(ctx context.Context, initiator uuid.UUID, peer uuid.UUID) (uuid.UUID, error)
	ListDialogs(ctx context.Context, userID uuid.UUID, limit int) ([]Dialog, error)
	CheckMember(ctx context.Context, dialogID, userID uuid.UUID) (bool, error)
	Members(ctx context.Context, dialogID uuid.UUID) ([]uuid.UUID, error)
	SaveMessage(ctx context.Context, dialogID, sender uuid.UUID, text string) (int64, time.Time, error)
	ListMessages(ctx context.Context, dialogID, userID uuid.UUID, limit int, before int64) ([]Message, error)
	MarkDelivered(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64) error
	MarkRead(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) CheckMember(ctx context.Context, dialogID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM dialog_members
			WHERE dialog_id = $1 AND user_id = $2
		)`, dialogID, userID).Scan(&exists)
	return exists, err
}

func (r *pgRepository) CreateDirect(ctx context.Context, initiator uuid.UUID, peer uuid.UUID) (uuid.UUID, error) {
	dialogID := uuid.New()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO dialogs (id, kind, is_encrypted, created_at, updated_at)
		VALUES ($1, 'direct', TRUE, NOW(), NOW())`, dialogID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, uid := range []uuid.UUID{initiator, peer} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO dialog_members (dialog_id, user_id, role)
			VALUES ($1, $2, 'member')`, dialogID, uid); err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return dialogID, nil
}

func (r *pgRepository) GetOrCreateDirect(ctx context.Context, initiator uuid.UUID, peer uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT d.id FROM dialogs d
		JOIN dialog_members m1 ON m1.dialog_id = d.id AND m1.user_id = $1
		JOIN dialog_members m2 ON m2.dialog_id = d.id AND m2.user_id = $2
		WHERE d.kind = 'direct'
		LIMIT 1`, initiator, peer).Scan(&id)
	if err == nil {
		return id, ErrDialogExist
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	return r.CreateDirect(ctx, initiator, peer)
}

func (r *pgRepository) ListDialogs(ctx context.Context, userID uuid.UUID, limit int) ([]Dialog, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
WITH last_msg AS (
  SELECT DISTINCT ON (dialog_id) dialog_id, id, sender_id, created_at, convert_from(cipher_text, 'UTF8') AS text
  FROM messages
  ORDER BY dialog_id, id DESC
),
unreads AS (
  SELECT dialog_id, COUNT(*) AS unread
  FROM messages m
  WHERE NOT EXISTS (
    SELECT 1 FROM message_reads r WHERE r.message_id = m.id AND r.user_id = $1
  ) AND m.sender_id <> $1
  GROUP BY dialog_id
)
SELECT d.id, COALESCE(d.title,''), lm.id, lm.sender_id, lm.created_at, lm.text,
       COALESCE(u.unread,0)
FROM dialogs d
JOIN dialog_members dm ON dm.dialog_id = d.id AND dm.user_id = $1
LEFT JOIN last_msg lm ON lm.dialog_id = d.id
LEFT JOIN unreads u ON u.dialog_id = d.id
ORDER BY lm.created_at DESC NULLS LAST, d.created_at DESC
LIMIT $2
`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Dialog
	for rows.Next() {
		var (
			id       uuid.UUID
			title    string
			msgID    *int64
			senderID *uuid.UUID
			created  *time.Time
			text     *string
			unread   int64
		)
		if err := rows.Scan(&id, &title, &msgID, &senderID, &created, &text, &unread); err != nil {
			return nil, err
		}
		var last *Message
		if msgID != nil && senderID != nil && created != nil && text != nil {
			last = &Message{
				ID: *msgID, SenderID: *senderID, DialogID: id, Text: *text, CreatedAt: *created,
			}
		}
		res = append(res, Dialog{
			ID: id, Title: title, LastMessage: last, UnreadCount: unread,
		})
	}
	return res, rows.Err()
}

func (r *pgRepository) Members(ctx context.Context, dialogID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id FROM dialog_members WHERE dialog_id = $1
	`, dialogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *pgRepository) SaveMessage(ctx context.Context, dialogID, sender uuid.UUID, text string) (int64, time.Time, error) {
	var id int64
	var created time.Time
	err := r.pool.QueryRow(ctx, `
		INSERT INTO messages (dialog_id, sender_id, cipher_text, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, created_at
	`, dialogID, sender, []byte(text)).Scan(&id, &created)
	return id, created, err
}

func (r *pgRepository) ListMessages(ctx context.Context, dialogID, userID uuid.UUID, limit int, before int64) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
WITH other_member AS (
  SELECT user_id FROM dialog_members WHERE dialog_id = $1 AND user_id <> $2 LIMIT 1
),
filtered AS (
  SELECT *
  FROM messages
  WHERE dialog_id = $1 AND ($3 = 0 OR id < $3)
  ORDER BY id DESC
  LIMIT $4
)
SELECT m.id, m.sender_id, m.dialog_id, convert_from(m.cipher_text,'UTF8') AS text, m.created_at,
       EXISTS(SELECT 1 FROM message_deliveries d WHERE d.message_id = m.id AND d.user_id = $2) AS delivered_to_me,
       EXISTS(SELECT 1 FROM message_reads r WHERE r.message_id = m.id AND r.user_id = $2) AS read_by_me,
       EXISTS(SELECT 1 FROM message_deliveries d WHERE d.message_id = m.id AND d.user_id = COALESCE(om.user_id, $2)) AS delivered_peer,
       EXISTS(SELECT 1 FROM message_reads r WHERE r.message_id = m.id AND r.user_id = COALESCE(om.user_id, $2)) AS read_peer
FROM filtered m
LEFT JOIN other_member om ON true
ORDER BY m.id DESC
	`, dialogID, userID, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SenderID, &m.DialogID, &m.Text, &m.CreatedAt, &m.DeliveredToMe, &m.ReadByMe, &m.DeliveredPeer, &m.ReadPeer); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *pgRepository) MarkDelivered(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO message_deliveries (message_id, user_id, delivered_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT DO NOTHING
	`, messageID, userID)
	return err
}

func (r *pgRepository) MarkRead(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO message_reads (message_id, user_id, read_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT DO NOTHING
	`, messageID, userID)
	return err
}
