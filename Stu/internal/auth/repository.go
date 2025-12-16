package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrUserExists signals unique email conflict.
	ErrUserExists = errors.New("user already exists")
	// ErrUserNotFound signals missing user.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidCode signals wrong or expired verification code.
	ErrInvalidCode = errors.New("invalid verification code")
	// ErrSessionNotFound signals missing session.
	ErrSessionNotFound = errors.New("session not found")
)

// User represents an auth user.
type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    []byte
	IsActive        bool
	IsAdmin         bool
	BannedAt        *time.Time
	BanReason       *string
	AdminTOTPSecret *string
	CreatedAt       time.Time
}

// Session holds session data for refresh rotation.
type Session struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	DeviceID             uuid.UUID
	RefreshTokenHash     []byte
	LastRefreshTokenHash []byte
	ExpiresAt            time.Time
	RevokedAt            *time.Time
	RevokedReason        string
	AccessTokenHash      []byte
}

type Repository interface {
	CreateUser(ctx context.Context, email string, passwordHash []byte) (User, error)
	ActivateUser(ctx context.Context, userID uuid.UUID) error
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
	SaveVerificationCode(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt time.Time) error
	ValidateVerificationCode(ctx context.Context, email string, codeHash []byte) (User, error)
	CreateDevice(ctx context.Context, userID uuid.UUID, name, platform string) (uuid.UUID, error)
	CreateSession(ctx context.Context, userID, deviceID uuid.UUID, accessTokenHash, refreshTokenHash []byte, expiresAt time.Time, userAgent, ip string) (uuid.UUID, error)
	GetSessionByRefresh(ctx context.Context, refreshHash []byte) (Session, bool, error)
	UpdateSessionTokens(ctx context.Context, sessionID uuid.UUID, newAccessHash, newRefreshHash []byte, expiresAt time.Time) error
	RevokeSession(ctx context.Context, sessionID uuid.UUID, reason string) error
	RevokeUserSessions(ctx context.Context, userID uuid.UUID, reason string) error
	ValidateAccessToken(ctx context.Context, accessHash []byte) (Session, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) CreateUser(ctx context.Context, email string, passwordHash []byte) (User, error) {
	query := `
		INSERT INTO users (email, password_hash, is_active)
		VALUES ($1, $2, FALSE)
		RETURNING id, email, password_hash, is_active, created_at`
	var u User
	err := r.pool.QueryRow(ctx, query, email, passwordHash).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return User{}, ErrUserExists
		}
		return User{}, err
	}
	return u, nil
}

func (r *pgRepository) ActivateUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET is_active = TRUE, updated_at = NOW() WHERE id = $1`, userID)
	return err
}

func (r *pgRepository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	query := `SELECT id, email, password_hash, is_active, is_admin, banned_at, ban_reason, admin_totp_secret, created_at FROM users WHERE email = $1 AND is_deleted = FALSE LIMIT 1`
	var u User
	err := r.pool.QueryRow(ctx, query, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.IsAdmin, &u.BannedAt, &u.BanReason, &u.AdminTOTPSecret, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return u, err
}

func (r *pgRepository) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	query := `SELECT id, email, password_hash, is_active, is_admin, banned_at, ban_reason, admin_totp_secret, created_at FROM users WHERE id = $1 AND is_deleted = FALSE LIMIT 1`
	var u User
	err := r.pool.QueryRow(ctx, query, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.IsAdmin, &u.BannedAt, &u.BanReason, &u.AdminTOTPSecret, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return u, err
}

func (r *pgRepository) SaveVerificationCode(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO verification_codes (user_id, code_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, codeHash, expiresAt)
	return err
}

func (r *pgRepository) ValidateVerificationCode(ctx context.Context, email string, codeHash []byte) (User, error) {
	query := `
		SELECT vc.id, u.id, u.email, u.password_hash, u.is_active, u.created_at
		FROM verification_codes vc
		JOIN users u ON u.id = vc.user_id
		WHERE u.email = $1
		  AND vc.code_hash = $2
		  AND vc.consumed_at IS NULL
		  AND vc.expires_at > NOW()
		ORDER BY vc.created_at DESC
		LIMIT 1`
	var (
		codeID uuid.UUID
		u      User
	)
	err := r.pool.QueryRow(ctx, query, email, codeHash).Scan(&codeID, &u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidCode
		}
		return User{}, err
	}
	_, err = r.pool.Exec(ctx, `UPDATE verification_codes SET consumed_at = NOW() WHERE id = $1`, codeID)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *pgRepository) CreateDevice(ctx context.Context, userID uuid.UUID, name, platform string) (uuid.UUID, error) {
	query := `
		INSERT INTO devices (user_id, name, platform, last_seen)
		VALUES ($1, $2, $3, NOW())
		RETURNING id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, query, userID, name, platform).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (r *pgRepository) CreateSession(ctx context.Context, userID, deviceID uuid.UUID, accessTokenHash, refreshTokenHash []byte, expiresAt time.Time, userAgent, ip string) (uuid.UUID, error) {
	query := `
		INSERT INTO sessions (user_id, device_id, access_token_hash, refresh_token_hash, user_agent, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, query, userID, deviceID, accessTokenHash, refreshTokenHash, userAgent, ip, expiresAt).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// GetSessionByRefresh returns session and bool indicating whether hash matched last_refresh_token_hash (reuse).
func (r *pgRepository) GetSessionByRefresh(ctx context.Context, refreshHash []byte) (Session, bool, error) {
	var s Session
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, device_id, refresh_token_hash, last_refresh_token_hash, expires_at, revoked_at, revoked_reason, access_token_hash
		FROM sessions
		WHERE refresh_token_hash = $1
		LIMIT 1`, refreshHash).
		Scan(&s.ID, &s.UserID, &s.DeviceID, &s.RefreshTokenHash, &s.LastRefreshTokenHash, &s.ExpiresAt, &s.RevokedAt, &s.RevokedReason, &s.AccessTokenHash)
	if err == nil {
		return s, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, false, err
	}

	err = r.pool.QueryRow(ctx, `
		SELECT id, user_id, device_id, refresh_token_hash, last_refresh_token_hash, expires_at, revoked_at, revoked_reason, access_token_hash
		FROM sessions
		WHERE last_refresh_token_hash = $1
		LIMIT 1`, refreshHash).
		Scan(&s.ID, &s.UserID, &s.DeviceID, &s.RefreshTokenHash, &s.LastRefreshTokenHash, &s.ExpiresAt, &s.RevokedAt, &s.RevokedReason, &s.AccessTokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, false, ErrSessionNotFound
		}
		return Session{}, false, err
	}
	return s, true, nil
}

func (r *pgRepository) UpdateSessionTokens(ctx context.Context, sessionID uuid.UUID, newAccessHash, newRefreshHash []byte, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions
		SET last_refresh_token_hash = refresh_token_hash,
		    refresh_token_hash = $2,
		    access_token_hash = $1,
		    expires_at = $3,
		    rotated_at = NOW()
		WHERE id = $4
	`, newAccessHash, newRefreshHash, expiresAt, sessionID)
	return err
}

func (r *pgRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = NOW(),
		    revoked_reason = $2
		WHERE id = $1
	`, sessionID, reason)
	return err
}

func (r *pgRepository) RevokeUserSessions(ctx context.Context, userID uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = NOW(),
		    revoked_reason = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, reason)
	return err
}

func (r *pgRepository) ValidateAccessToken(ctx context.Context, accessHash []byte) (Session, error) {
	var s Session
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, device_id, refresh_token_hash, last_refresh_token_hash, expires_at, revoked_at, revoked_reason, access_token_hash
		FROM sessions
		WHERE access_token_hash = $1 AND revoked_at IS NULL
		LIMIT 1`, accessHash).
		Scan(&s.ID, &s.UserID, &s.DeviceID, &s.RefreshTokenHash, &s.LastRefreshTokenHash, &s.ExpiresAt, &s.RevokedAt, &s.RevokedReason, &s.AccessTokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	if s.ExpiresAt.Before(time.Now()) {
		return Session{}, fmt.Errorf("session expired")
	}
	return s, nil
}
