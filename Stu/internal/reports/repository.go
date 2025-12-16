package reports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Report struct {
	ID             uuid.UUID  `json:"id"`
	ReporterID     uuid.UUID  `json:"reporter_id"`
	ReportedUserID uuid.UUID  `json:"reported_user_id"`
	DialogID       *uuid.UUID `json:"dialog_id,omitempty"`
	MessageID      *int64     `json:"message_id,omitempty"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	AIVerdict      *string    `json:"ai_verdict,omitempty"`
	AIConfidence   *float64   `json:"ai_confidence,omitempty"`
	AINotes        *string    `json:"ai_notes,omitempty"`
	AnalyzedAt     *time.Time `json:"analyzed_at,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, rep Report) (Report, error)
	ListMine(ctx context.Context, userID uuid.UUID) ([]Report, error)
	ListAdmin(ctx context.Context, status string, limit, offset int) ([]ReportAdminView, error)
	UpdateAIResult(ctx context.Context, id uuid.UUID, verdict string, confidence float64, notes string) error
	GetMessageText(ctx context.Context, messageID int64) (string, error)
	Close(ctx context.Context, id uuid.UUID) error
}

type ReportAdminView struct {
	Report
	ReporterEmail string `json:"reporter_email"`
	ReportedEmail string `json:"reported_email"`
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) Create(ctx context.Context, rep Report) (Report, error) {
	var dialogID *uuid.UUID
	if rep.DialogID != nil {
		dialogID = rep.DialogID
	}
	var messageID *int64
	if rep.MessageID != nil {
		messageID = rep.MessageID
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO reports (reporter_id, reported_user_id, dialog_id, message_id, reason)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, status, created_at, ai_verdict, ai_confidence, ai_notes, analyzed_at
	`, rep.ReporterID, rep.ReportedUserID, dialogID, messageID, rep.Reason).Scan(&rep.ID, &rep.Status, &rep.CreatedAt, &rep.AIVerdict, &rep.AIConfidence, &rep.AINotes, &rep.AnalyzedAt)
	return rep, err
}

func (r *pgRepository) ListMine(ctx context.Context, userID uuid.UUID) ([]Report, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, reporter_id, reported_user_id, dialog_id, message_id, reason, status, created_at, ai_verdict, ai_confidence, ai_notes, analyzed_at
		FROM reports
		WHERE reporter_id = $1
		ORDER BY created_at DESC
		LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Report
	for rows.Next() {
		var rep Report
		var dialogID *uuid.UUID
		var msgID *int64
		if err := rows.Scan(&rep.ID, &rep.ReporterID, &rep.ReportedUserID, &dialogID, &msgID, &rep.Reason, &rep.Status, &rep.CreatedAt, &rep.AIVerdict, &rep.AIConfidence, &rep.AINotes, &rep.AnalyzedAt); err != nil {
			return nil, err
		}
		rep.DialogID = dialogID
		rep.MessageID = msgID
		res = append(res, rep)
	}
	return res, rows.Err()
}

func (r *pgRepository) ListAdmin(ctx context.Context, status string, limit, offset int) ([]ReportAdminView, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT rp.id, rp.reporter_id, rp.reported_user_id, rp.dialog_id, rp.message_id, rp.reason, rp.status, rp.created_at,
		       rp.ai_verdict, rp.ai_confidence, rp.ai_notes, rp.analyzed_at,
		       rep.email, rud.email
		FROM reports rp
		JOIN users rep ON rep.id = rp.reporter_id
		JOIN users rud ON rud.id = rp.reported_user_id
		WHERE ($1 = '' OR rp.status = $1)
		ORDER BY rp.created_at DESC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ReportAdminView
	for rows.Next() {
		var view ReportAdminView
		var dialogID *uuid.UUID
		var msgID *int64
		if err := rows.Scan(&view.ID, &view.ReporterID, &view.ReportedUserID, &dialogID, &msgID, &view.Reason, &view.Status, &view.CreatedAt, &view.AIVerdict, &view.AIConfidence, &view.AINotes, &view.AnalyzedAt, &view.ReporterEmail, &view.ReportedEmail); err != nil {
			return nil, err
		}
		view.DialogID = dialogID
		view.MessageID = msgID
		res = append(res, view)
	}
	return res, rows.Err()
}

func (r *pgRepository) UpdateAIResult(ctx context.Context, id uuid.UUID, verdict string, confidence float64, notes string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE reports
		SET ai_verdict = $2,
		    ai_confidence = $3,
		    ai_notes = $4,
		    analyzed_at = NOW()
		WHERE id = $1
	`, id, verdict, confidence, notes)
	return err
}

func (r *pgRepository) GetMessageText(ctx context.Context, messageID int64) (string, error) {
	var text string
	err := r.pool.QueryRow(ctx, `
		SELECT convert_from(cipher_text,'UTF8') AS text
		FROM messages
		WHERE id = $1
	`, messageID).Scan(&text)
	return text, err
}

func (r *pgRepository) Close(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE reports SET status = 'closed' WHERE id = $1
	`, id)
	return err
}
