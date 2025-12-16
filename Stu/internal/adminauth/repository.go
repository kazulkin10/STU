package adminauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"stu/internal/security"
)

// Session represents an admin auth session state machine.
type Session struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Token        string
	State        string
	TotpVerified bool
	EmailCode    string
	EmailExpires *time.Time
	ExpiresAt    time.Time
}

// Repository handles admin auth sessions and secrets.
type Repository interface {
	CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (Session, error)
	GetByToken(ctx context.Context, token string) (Session, error)
	MarkTotpVerified(ctx context.Context, token string) error
	SetEmailCode(ctx context.Context, token, code string, expires time.Time) error
	DeleteSession(ctx context.Context, token string) error
	SetTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (Session, error) {
	token, hash, err := security.GenerateOpaqueToken()
	if err != nil {
		return Session{}, err
	}
	hashStr := base64.RawStdEncoding.EncodeToString(hash)
	expires := time.Now().Add(ttl)
	var id uuid.UUID
	err = r.pool.QueryRow(ctx, `
		INSERT INTO admin_auth_sessions (user_id, session_token, state, expires_at)
		VALUES ($1, $2, 'password_ok', $3)
		RETURNING id
	`, userID, hashStr, expires).Scan(&id)
	if err != nil {
		return Session{}, err
	}
	return Session{
		ID:        id,
		UserID:    userID,
		Token:     token,
		State:     "password_ok",
		ExpiresAt: expires,
	}, nil
}

func (r *pgRepository) GetByToken(ctx context.Context, token string) (Session, error) {
	hash := sha256.Sum256([]byte(token))
	hashStr := base64.RawStdEncoding.EncodeToString(hash[:])
	var s Session
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, state, totp_verified, COALESCE(email_code,''), email_expires_at, expires_at
		FROM admin_auth_sessions
		WHERE session_token = $1
	`, hashStr).Scan(&s.ID, &s.UserID, &s.State, &s.TotpVerified, &s.EmailCode, &s.EmailExpires, &s.ExpiresAt)
	if err != nil {
		return Session{}, err
	}
	s.Token = token
	return s, nil
}

func (r *pgRepository) MarkTotpVerified(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	hashStr := base64.RawStdEncoding.EncodeToString(hash[:])
	_, err := r.pool.Exec(ctx, `
		UPDATE admin_auth_sessions
		SET totp_verified = TRUE, state = 'totp_ok'
		WHERE session_token = $1
	`, hashStr)
	return err
}

func (r *pgRepository) SetEmailCode(ctx context.Context, token, code string, expires time.Time) error {
	hash := sha256.Sum256([]byte(token))
	hashStr := base64.RawStdEncoding.EncodeToString(hash[:])
	_, err := r.pool.Exec(ctx, `
		UPDATE admin_auth_sessions
		SET email_code = $2, email_expires_at = $3
		WHERE session_token = $1
	`, hashStr, code, expires)
	return err
}

func (r *pgRepository) DeleteSession(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	hashStr := base64.RawStdEncoding.EncodeToString(hash[:])
	_, err := r.pool.Exec(ctx, `DELETE FROM admin_auth_sessions WHERE session_token = $1`, hashStr)
	return err
}

func (r *pgRepository) SetTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET admin_totp_secret = $2 WHERE id = $1
	`, userID, secret)
	return err
}
