package auth

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubMailer struct {
	lastCode string
	fail     bool
}

func (m *stubMailer) SendVerification(toEmail, code string) error {
	if m.fail {
		return ErrInvalidCode
	}
	m.lastCode = code
	return nil
}

// inMemoryRepo is a lightweight repo for service tests.
type inMemoryRepo struct {
	users        map[string]User
	codes        map[string][]codeEntry
	sessions     map[uuid.UUID]Session
	refreshIndex map[string]uuid.UUID
	lastIndex    map[string]uuid.UUID
}

type codeEntry struct {
	userID    uuid.UUID
	codeHash  []byte
	expiresAt time.Time
	consumed  bool
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{
		users:        make(map[string]User),
		codes:        make(map[string][]codeEntry),
		sessions:     make(map[uuid.UUID]Session),
		refreshIndex: make(map[string]uuid.UUID),
		lastIndex:    make(map[string]uuid.UUID),
	}
}

func key(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func (r *inMemoryRepo) CreateUser(ctx context.Context, email string, passwordHash []byte) (User, error) {
	if _, exists := r.users[email]; exists {
		return User{}, ErrUserExists
	}
	u := User{ID: uuid.New(), Email: email, PasswordHash: passwordHash, IsActive: false, CreatedAt: time.Now()}
	r.users[email] = u
	return u, nil
}

func (r *inMemoryRepo) ActivateUser(ctx context.Context, userID uuid.UUID) error {
	for email, u := range r.users {
		if u.ID == userID {
			u.IsActive = true
			r.users[email] = u
			return nil
		}
	}
	return ErrUserNotFound
}

func (r *inMemoryRepo) GetUserByEmail(ctx context.Context, email string) (User, error) {
	u, ok := r.users[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (r *inMemoryRepo) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (r *inMemoryRepo) SaveVerificationCode(ctx context.Context, userID uuid.UUID, codeHash []byte, expiresAt time.Time) error {
	for email, u := range r.users {
		if u.ID == userID {
			r.codes[email] = append(r.codes[email], codeEntry{userID: userID, codeHash: codeHash, expiresAt: expiresAt})
			return nil
		}
	}
	return ErrUserNotFound
}

func (r *inMemoryRepo) ValidateVerificationCode(ctx context.Context, email string, codeHash []byte) (User, error) {
	entries := r.codes[email]
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.consumed {
			continue
		}
		if entry.expiresAt.Before(time.Now()) {
			continue
		}
		if key(entry.codeHash) == key(codeHash) {
			entry.consumed = true
			entries[i] = entry
			r.codes[email] = entries
			return r.users[email], nil
		}
	}
	return User{}, ErrInvalidCode
}

func (r *inMemoryRepo) CreateDevice(ctx context.Context, userID uuid.UUID, name, platform string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (r *inMemoryRepo) CreateSession(ctx context.Context, userID, deviceID uuid.UUID, accessTokenHash, refreshTokenHash []byte, expiresAt time.Time, userAgent, ip string) (uuid.UUID, error) {
	id := uuid.New()
	s := Session{
		ID:               id,
		UserID:           userID,
		DeviceID:         deviceID,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        expiresAt,
	}
	r.sessions[id] = s
	r.refreshIndex[key(refreshTokenHash)] = id
	return id, nil
}

func (r *inMemoryRepo) GetSessionByRefresh(ctx context.Context, refreshHash []byte) (Session, bool, error) {
	if id, ok := r.refreshIndex[key(refreshHash)]; ok {
		return r.sessions[id], false, nil
	}
	if id, ok := r.lastIndex[key(refreshHash)]; ok {
		return r.sessions[id], true, nil
	}
	return Session{}, false, ErrSessionNotFound
}

func (r *inMemoryRepo) UpdateSessionTokens(ctx context.Context, sessionID uuid.UUID, newAccessHash, newRefreshHash []byte, expiresAt time.Time) error {
	s, ok := r.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	delete(r.refreshIndex, key(s.RefreshTokenHash))
	s.LastRefreshTokenHash = s.RefreshTokenHash
	r.lastIndex[key(s.LastRefreshTokenHash)] = sessionID
	s.RefreshTokenHash = newRefreshHash
	s.ExpiresAt = expiresAt
	r.sessions[sessionID] = s
	r.refreshIndex[key(newRefreshHash)] = sessionID
	return nil
}

func (r *inMemoryRepo) RevokeSession(ctx context.Context, sessionID uuid.UUID, reason string) error {
	s, ok := r.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	now := time.Now()
	s.RevokedAt = &now
	s.RevokedReason = reason
	r.sessions[sessionID] = s
	return nil
}

func (r *inMemoryRepo) RevokeUserSessions(ctx context.Context, userID uuid.UUID, reason string) error {
	for id, s := range r.sessions {
		if s.UserID == userID {
			now := time.Now()
			s.RevokedAt = &now
			s.RevokedReason = reason
			r.sessions[id] = s
		}
	}
	return nil
}

func (r *inMemoryRepo) ValidateAccessToken(ctx context.Context, accessHash []byte) (Session, error) {
	if id, ok := r.refreshIndex[key(accessHash)]; ok {
		return r.sessions[id], nil
	}
	return Session{}, ErrSessionNotFound
}

func TestRegisterVerifyRefreshFlow(t *testing.T) {
	repo := newInMemoryRepo()
	mail := &stubMailer{}
	svc := NewService(repo, mail, Config{
		AccessTokenTTL:      time.Minute * 15,
		RefreshTokenTTL:     time.Hour,
		VerificationCodeTTL: time.Minute * 15,
	})

	ctx := context.Background()
	userID, err := svc.Register(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if userID == uuid.Nil {
		t.Fatalf("user id is nil")
	}
	if mail.lastCode == "" {
		t.Fatalf("verification code not sent")
	}

	_, deviceID, access, refresh, err := svc.Verify(ctx, "test@example.com", mail.lastCode, "pc", "windows", "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("verify error: %v", err)
	}
	if access == "" || refresh == "" || deviceID == uuid.Nil {
		t.Fatalf("expected tokens and device")
	}

	newAccess, newRefresh, err := svc.Refresh(ctx, refresh, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("refresh error: %v", err)
	}
	if newAccess == access || newRefresh == refresh {
		t.Fatalf("tokens not rotated")
	}

	// Reuse old refresh should revoke session
	if _, _, err := svc.Refresh(ctx, refresh, "ua", "127.0.0.1"); err != ErrRefreshReuse {
		t.Fatalf("expected reuse error, got %v", err)
	}

	// Logout all should revoke
	if err := svc.LogoutAll(ctx, newRefresh); err != nil {
		t.Fatalf("logout all error: %v", err)
	}
	if _, _, err := svc.Refresh(ctx, newRefresh, "ua", "127.0.0.1"); err == nil {
		t.Fatalf("expected refresh failure after revoke")
	}
}

func TestLoginInactiveFails(t *testing.T) {
	repo := newInMemoryRepo()
	mail := &stubMailer{}
	svc := NewService(repo, mail, Config{
		AccessTokenTTL:      time.Minute * 15,
		RefreshTokenTTL:     time.Hour,
		VerificationCodeTTL: time.Minute * 15,
	})
	ctx := context.Background()
	_, err := svc.Register(ctx, "inactive@example.com", "password123")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if _, _, _, _, err := svc.Login(ctx, "inactive@example.com", "password123", "pc", "windows", "ua", ""); err != ErrInactive {
		t.Fatalf("expected inactive error, got %v", err)
	}
}
