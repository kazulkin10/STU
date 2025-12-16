package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog"

	"stu/internal/auth"
	"stu/internal/mailer"
	"stu/internal/security"
)

var (
	ErrNotAdmin       = errors.New("not_admin")
	ErrInvalidStep    = errors.New("invalid_step")
	ErrInvalidCode    = errors.New("invalid_code")
	ErrSessionExpired = errors.New("session_expired")
)

// Service manages admin multi-factor auth.
type Service struct {
	authRepo  auth.Repository
	adminRepo Repository
	authSvc   *auth.Service
	mailer    *mailer.Mailer
	logger    zerolog.Logger
}

func NewService(authRepo auth.Repository, adminRepo Repository, authSvc *auth.Service, mailer *mailer.Mailer, logger zerolog.Logger) *Service {
	return &Service{
		authRepo:  authRepo,
		adminRepo: adminRepo,
		authSvc:   authSvc,
		mailer:    mailer,
		logger:    logger,
	}
}

// Login step validates password for admin and starts session.
func (s *Service) Login(ctx context.Context, email, password, deviceName, platform, userAgent, ip string) (string, error) {
	user, err := s.authRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if !user.IsAdmin {
		return "", ErrNotAdmin
	}
	if user.BannedAt != nil {
		s.authSvc.SetBanInfo(user.BanReason, user.BannedAt)
		return "", auth.ErrBanned
	}
	if !security.CheckPassword(user.PasswordHash, password) {
		return "", auth.ErrInvalidCredentials
	}
	session, err := s.adminRepo.CreateSession(ctx, user.ID, 15*time.Minute)
	if err != nil {
		return "", err
	}
	return session.Token, nil
}

// InitTOTP generates secret once for admin.
func (s *Service) InitTOTP(ctx context.Context, token string) (string, error) {
	sess, err := s.adminRepo.GetByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if time.Now().After(sess.ExpiresAt) {
		return "", ErrSessionExpired
	}
	user, err := s.authRepo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return "", err
	}
	if user.AdminTOTPSecret != nil && *user.AdminTOTPSecret != "" {
		return *user.AdminTOTPSecret, nil
	}
	secretBytes := make([]byte, 10)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", err
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	if err := s.adminRepo.SetTOTPSecret(ctx, user.ID, secret); err != nil {
		return "", err
	}
	return secret, nil
}

// VerifyTOTP validates TOTP and triggers email code.
func (s *Service) VerifyTOTP(ctx context.Context, token, code string) (string, error) {
	sess, err := s.adminRepo.GetByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if time.Now().After(sess.ExpiresAt) {
		return "", ErrSessionExpired
	}
	user, err := s.authRepo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return "", err
	}
	if user.AdminTOTPSecret == nil || *user.AdminTOTPSecret == "" {
		return "", ErrInvalidStep
	}
	if !totp.Validate(code, *user.AdminTOTPSecret) {
		return "", ErrInvalidCode
	}
	if err := s.adminRepo.MarkTotpVerified(ctx, token); err != nil {
		return "", err
	}
	emailCode := generateCode()
	exp := time.Now().Add(10 * time.Minute)
	if err := s.adminRepo.SetEmailCode(ctx, token, emailCode, exp); err != nil {
		return "", err
	}
	if s.mailer != nil {
		if err := s.mailer.SendAdminCode(user.Email, emailCode); err != nil {
			s.logger.Warn().Err(err).Msg("send admin code failed")
		}
	}
	return "email_code_sent", nil
}

// VerifyEmail finalizes MFA and issues access/refresh tokens.
func (s *Service) VerifyEmail(ctx context.Context, token, code, deviceName, platform, userAgent, ip string) (authTokens, error) {
	sess, err := s.adminRepo.GetByToken(ctx, token)
	if err != nil {
		return authTokens{}, err
	}
	if time.Now().After(sess.ExpiresAt) {
		return authTokens{}, ErrSessionExpired
	}
	if !sess.TotpVerified {
		return authTokens{}, ErrInvalidStep
	}
	if sess.EmailCode == "" || sess.EmailExpires == nil || time.Now().After(*sess.EmailExpires) {
		return authTokens{}, ErrInvalidCode
	}
	if sess.EmailCode != code {
		return authTokens{}, ErrInvalidCode
	}
	user, err := s.authRepo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return authTokens{}, err
	}
	if user.BannedAt != nil {
		s.authSvc.SetBanInfo(user.BanReason, user.BannedAt)
		return authTokens{}, auth.ErrBanned
	}
	uid, deviceID, access, refresh, err := s.authSvc.IssueSessionForUser(ctx, user, deviceName, platform, userAgent, ip)
	if err != nil {
		return authTokens{}, err
	}
	_ = s.adminRepo.DeleteSession(ctx, token)
	return authTokens{
		UserID:       uid,
		DeviceID:     deviceID,
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (s *Service) Logout(ctx context.Context, refresh string) error {
	return s.authSvc.Logout(ctx, refresh)
}

type authTokens struct {
	UserID       uuid.UUID
	DeviceID     uuid.UUID
	AccessToken  string
	RefreshToken string
}

// BanInfo exposes last ban details (from auth service).
func (s *Service) BanInfo() (*string, *time.Time) {
	return s.authSvc.LastBanInfo()
}

func generateCode() string {
	raw := make([]byte, 3)
	_, _ = rand.Read(raw)
	sum := sha256.Sum256(raw)
	n := int(sum[0])<<16 | int(sum[1])<<8 | int(sum[2])
	return fmt.Sprintf("%06d", n%1000000)
}
