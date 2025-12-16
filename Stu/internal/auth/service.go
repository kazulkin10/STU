package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"

	"stu/internal/security"
)

// Config controls auth TTLs.
type Config struct {
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	VerificationCodeTTL time.Duration
}

// CodeSender abstracts verification email sending.
type CodeSender interface {
	SendVerification(toEmail, code string) error
}

var (
	// ErrInvalidCredentials signals wrong password.
	ErrInvalidCredentials = fmt.Errorf("invalid credentials")
	// ErrInactive signals account not verified.
	ErrInactive = fmt.Errorf("account not verified")
	// ErrRefreshReuse signals refresh token reuse.
	ErrRefreshReuse = fmt.Errorf("refresh token reused and session revoked")
	// ErrSessionRevoked signals revoked session.
	ErrSessionRevoked = fmt.Errorf("session revoked")
	// ErrBanned signals banned user.
	ErrBanned = fmt.Errorf("banned")
)

// Service handles registration/login flows.
type Service struct {
	repo       Repository
	config     Config
	codeSender CodeSender
	// Banned error detail (populated after repo calls)
	banReason *string
	banAt     *time.Time
}

func NewService(repo Repository, sender CodeSender, cfg Config) *Service {
	return &Service{repo: repo, codeSender: sender, config: cfg}
}

// Register creates user and sends verification code.
func (s *Service) Register(ctx context.Context, email, password string) (uuid.UUID, error) {
	passHash, err := security.HashPassword(password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, email, passHash)
	if err != nil {
		return uuid.Nil, err
	}

	code, codeHash, err := generateCode()
	if err != nil {
		return uuid.Nil, fmt.Errorf("code generate: %w", err)
	}
	if err := s.repo.SaveVerificationCode(ctx, user.ID, codeHash, time.Now().Add(s.config.VerificationCodeTTL)); err != nil {
		return uuid.Nil, fmt.Errorf("save code: %w", err)
	}
	if err := s.codeSender.SendVerification(email, code); err != nil {
		return uuid.Nil, fmt.Errorf("send code: %w", err)
	}
	return user.ID, nil
}

// Verify activates account and issues session.
func (s *Service) Verify(ctx context.Context, email, code, deviceName, platform, userAgent, ip string) (uuid.UUID, uuid.UUID, string, string, error) {
	codeHash := hashCode(code)
	user, err := s.repo.ValidateVerificationCode(ctx, email, codeHash)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", "", err
	}
	if user.BannedAt != nil {
		s.banReason = user.BanReason
		s.banAt = user.BannedAt
		return uuid.Nil, uuid.Nil, "", "", ErrBanned
	}
	if err := s.repo.ActivateUser(ctx, user.ID); err != nil {
		return uuid.Nil, uuid.Nil, "", "", err
	}
	return s.issueSession(ctx, user, deviceName, platform, userAgent, ip)
}

// Login verifies credentials and issues new session for active user.
func (s *Service) Login(ctx context.Context, email, password, deviceName, platform, userAgent, ip string) (uuid.UUID, uuid.UUID, string, string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", "", err
	}
	if !user.IsActive {
		return uuid.Nil, uuid.Nil, "", "", ErrInactive
	}
	if user.BannedAt != nil {
		s.banReason = user.BanReason
		s.banAt = user.BannedAt
		return uuid.Nil, uuid.Nil, "", "", ErrBanned
	}
	if !security.CheckPassword(user.PasswordHash, password) {
		return uuid.Nil, uuid.Nil, "", "", ErrInvalidCredentials
	}
	return s.issueSession(ctx, user, deviceName, platform, userAgent, ip)
}

// Refresh rotates tokens and detects reuse.
func (s *Service) Refresh(ctx context.Context, refreshToken string, userAgent, ip string) (string, string, error) {
	refreshHash := sha256.Sum256([]byte(refreshToken))
	session, matchedPrev, err := s.repo.GetSessionByRefresh(ctx, refreshHash[:])
	if err != nil {
		return "", "", err
	}
	if session.RevokedAt != nil {
		return "", "", ErrSessionRevoked
	}
	if matchedPrev {
		_ = s.repo.RevokeSession(ctx, session.ID, "refresh_reuse")
		return "", "", ErrRefreshReuse
	}
	if session.ExpiresAt.Before(time.Now()) {
		return "", "", fmt.Errorf("refresh expired")
	}

	newAccess, newAccessHash, err := security.GenerateOpaqueToken()
	if err != nil {
		return "", "", err
	}
	newRefresh, newRefreshHash, err := security.GenerateOpaqueToken()
	if err != nil {
		return "", "", err
	}
	expiresAt := time.Now().Add(s.config.RefreshTokenTTL)
	if err := s.repo.UpdateSessionTokens(ctx, session.ID, newAccessHash, newRefreshHash, expiresAt); err != nil {
		return "", "", err
	}
	return newAccess, newRefresh, nil
}

// Logout revokes session by refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	refreshHash := sha256.Sum256([]byte(refreshToken))
	session, matchedPrev, err := s.repo.GetSessionByRefresh(ctx, refreshHash[:])
	if err != nil {
		return err
	}
	if matchedPrev {
		return ErrRefreshReuse
	}
	return s.repo.RevokeSession(ctx, session.ID, "logout")
}

// LogoutAll revokes all sessions for the user owning the refresh token.
func (s *Service) LogoutAll(ctx context.Context, refreshToken string) error {
	refreshHash := sha256.Sum256([]byte(refreshToken))
	session, _, err := s.repo.GetSessionByRefresh(ctx, refreshHash[:])
	if err != nil {
		return err
	}
	return s.repo.RevokeUserSessions(ctx, session.UserID, "logout_all")
}

// LastBanInfo returns last ban reason/at populated during Login/Verify.
func (s *Service) LastBanInfo() (*string, *time.Time) {
	return s.banReason, s.banAt
}

// SetBanInfo allows other flows (admin auth) to propagate ban details.
func (s *Service) SetBanInfo(reason *string, at *time.Time) {
	s.banReason = reason
	s.banAt = at
}

func (s *Service) issueSession(ctx context.Context, user User, deviceName, platform, userAgent, ip string) (uuid.UUID, uuid.UUID, string, string, error) {
	deviceID, err := s.repo.CreateDevice(ctx, user.ID, deviceNameOrDefault(deviceName), platform)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", "", fmt.Errorf("create device: %w", err)
	}

	accessToken, accessHash, err := security.GenerateOpaqueToken()
	if err != nil {
		return uuid.Nil, uuid.Nil, "", "", err
	}
	refreshToken, refreshHash, err := security.GenerateOpaqueToken()
	if err != nil {
		return uuid.Nil, uuid.Nil, "", "", err
	}

	expiresAt := time.Now().Add(s.config.RefreshTokenTTL)
	if _, err = s.repo.CreateSession(ctx, user.ID, deviceID, accessHash, refreshHash, expiresAt, userAgent, ip); err != nil {
		return uuid.Nil, uuid.Nil, "", "", fmt.Errorf("create session: %w", err)
	}
	return user.ID, deviceID, accessToken, refreshToken, nil
}

// IssueSessionForUser issues tokens for already-authenticated user (used in admin MFA).
func (s *Service) IssueSessionForUser(ctx context.Context, user User, deviceName, platform, userAgent, ip string) (uuid.UUID, uuid.UUID, string, string, error) {
	if user.BannedAt != nil {
		s.banReason = user.BanReason
		s.banAt = user.BannedAt
		return uuid.Nil, uuid.Nil, "", "", ErrBanned
	}
	return s.issueSession(ctx, user, deviceName, platform, userAgent, ip)
}

func deviceNameOrDefault(name string) string {
	if name == "" {
		return "unknown-device"
	}
	return name
}

func generateCode() (string, []byte, error) {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", nil, err
	}
	code := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	codeStr := fmt.Sprintf("%06d", code%1000000)
	hash := hashCode(codeStr)
	return codeStr, hash, nil
}

func hashCode(code string) []byte {
	sum := sha256.Sum256([]byte(code))
	return sum[:]
}
