package auth

import (
	"context"
	"time"
)

// SessionInfo is a lightweight view for realtime.
type SessionInfo struct {
	UserID   string
	DeviceID string
	IsAdmin  bool
	Banned   bool
	BanReason *string
	BanAt     *time.Time
}

// AccessValidatorImpl wraps Repository.ValidateAccessToken.
type AccessValidatorImpl struct {
	repo Repository
}

func NewAccessValidator(repo Repository) *AccessValidatorImpl {
	return &AccessValidatorImpl{repo: repo}
}

func (v *AccessValidatorImpl) ValidateAccessToken(ctx context.Context, hash []byte) (SessionInfo, error) {
	s, err := v.repo.ValidateAccessToken(ctx, hash)
	if err != nil {
		return SessionInfo{}, err
	}
	if s.ExpiresAt.Before(time.Now()) {
		return SessionInfo{}, ErrSessionNotFound
	}
	// ban check via user lookup
	user, err := v.repo.GetUserByID(ctx, s.UserID)
	if err != nil {
		return SessionInfo{}, err
	}
	if user.BannedAt != nil {
		return SessionInfo{
			UserID:    s.UserID.String(),
			DeviceID:  s.DeviceID.String(),
			Banned:    true,
			BanReason: user.BanReason,
			BanAt:     user.BannedAt,
		}, ErrBanned
	}
	return SessionInfo{
		UserID:    s.UserID.String(),
		DeviceID:  s.DeviceID.String(),
		IsAdmin:   user.IsAdmin,
		Banned:    false,
		BanReason: user.BanReason,
		BanAt:     user.BannedAt,
	}, nil
}
