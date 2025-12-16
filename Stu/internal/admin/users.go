package admin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UsersRepo provides ban/unban operations.
type UsersRepo struct {
	pool *pgxpool.Pool
}

func NewUsersRepo(pool *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{pool: pool}
}

func (r *UsersRepo) Ban(ctx context.Context, userID uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET banned_at = $2, ban_reason = $3 WHERE id = $1`, userID, time.Now(), reason)
	return err
}

func (r *UsersRepo) Unban(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET banned_at = NULL, ban_reason = NULL WHERE id = $1`, userID)
	return err
}
