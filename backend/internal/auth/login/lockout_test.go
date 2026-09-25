package login

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type lockoutRepository struct {
	users  map[string]User
	audits []AuditEvent
}

func (r *lockoutRepository) WithinLoginTransaction(_ context.Context, fn func(Writer) error) error {
	return fn(r)
}
func (r *lockoutRepository) GetLoginUserByEmail(_ context.Context, email string) (User, error) {
	user, ok := r.users[email]
	if !ok {
		return User{}, pgx.ErrNoRows
	}
	return user, nil
}
func (r *lockoutRepository) CountLoginFailuresByAccount(_ context.Context, userID uuid.UUID, _ time.Time) (int64, error) {
	var count int64
	for _, audit := range r.audits {
		if audit.Action == "login_failed" && audit.ActorUserID != nil && *audit.ActorUserID == userID {
			count++
		}
	}
	return count, nil
}
func (r *lockoutRepository) CountLoginFailuresByIP(_ context.Context, ip netip.Addr, _ time.Time) (int64, error) {
	var count int64
	for _, audit := range r.audits {
		if audit.Action == "login_failed" && audit.IP != nil && *audit.IP == ip {
			count++
		}
	}
	return count, nil
}
func (r *lockoutRepository) LockLoginUser(_ context.Context, userID uuid.UUID, until time.Time) error {
	for email, user := range r.users {
		if user.ID == userID {
			user.Status = StatusLocked
			user.LockedUntil = &until
			r.users[email] = user
		}
	}
	return nil
}
func (r *lockoutRepository) UnlockLoginUser(_ context.Context, userID uuid.UUID) error {
	for email, user := range r.users {
		if user.ID == userID {
			user.Status = StatusActive
			user.LockedUntil = nil
			r.users[email] = user
		}
	}
	return nil
}
func (r *lockoutRepository) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	return []string{"user"}, nil
}
func (r *lockoutRepository) UpdateLoginSuccess(context.Context, uuid.UUID, string) error { return nil }
func (r *lockoutRepository) CreateRefreshToken(context.Context, RefreshToken) error      { return nil }
func (r *lockoutRepository) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

func newLockoutService(t *testing.T, repository Repository, policy LockoutConfig) *Service {
	t.Helper()
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return New(repository, signer, time.Hour, policy)
}

type stubSecurityEventPublisher struct{ events []SecurityEvent }

func (p *stubSecurityEventPublisher) PublishSecurityEvent(_ context.Context, event SecurityEvent) error {
	p.events = append(p.events, event)
	return nil
}

func activeUser(t *testing.T, email string) User {
	t.Helper()
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	return User{ID: uuid.New(), Email: email, PasswordHash: hash, Status: StatusActive}
}

func TestRF017_ContrasenaCorrectaFallaDuranteElBloqueo(t *testing.T) {
	now := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	user := activeUser(t, "ada@example.com")
	repository := &lockoutRepository{users: map[string]User{user.Email: user}}
	service := newLockoutService(t, repository, LockoutConfig{AccountMaxFailures: 1, IPMaxFailures: 20, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute})
	service.now = func() time.Time { return now }

	if _, err := service.Login(context.Background(), Input{Email: user.Email, Password: "wrong password"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("initial failed login error = %v", err)
	}
	if _, err := service.Login(context.Background(), Input{Email: user.Email, Password: "correct horse battery"}); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("locked correct-password login error = %v, want account locked", err)
	}
}

func TestRF017_BloqueoSeLevantaAlExpirar(t *testing.T) {
	now := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	user := activeUser(t, "ada@example.com")
	repository := &lockoutRepository{users: map[string]User{user.Email: user}}
	service := newLockoutService(t, repository, LockoutConfig{AccountMaxFailures: 1, IPMaxFailures: 20, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute})
	service.now = func() time.Time { return now }
	if _, err := service.Login(context.Background(), Input{Email: user.Email, Password: "wrong password"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("initial failed login error = %v", err)
	}
	now = now.Add(15*time.Minute + time.Nanosecond)
	if _, err := service.Login(context.Background(), Input{Email: user.Email, Password: "correct horse battery"}); err != nil {
		t.Fatalf("expired lock login error = %v", err)
	}
	stored := repository.users[user.Email]
	if stored.Status != StatusActive || stored.LockedUntil != nil {
		t.Fatalf("expired lock was not cleared: %+v", stored)
	}
}

func TestRF017_LimitePorIPBloqueaTrasElUmbral(t *testing.T) {
	first := activeUser(t, "first@example.com")
	second := activeUser(t, "second@example.com")
	repository := &lockoutRepository{users: map[string]User{first.Email: first, second.Email: second}}
	service := newLockoutService(t, repository, LockoutConfig{AccountMaxFailures: 5, IPMaxFailures: 3, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute})
	ip := netip.MustParseAddr("198.51.100.10")
	for _, email := range []string{first.Email, second.Email, "missing@example.com"} {
		if _, err := service.Login(context.Background(), Input{Email: email, Password: "wrong password", IP: &ip}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("failed login for %s error = %v", email, err)
		}
	}
	if _, err := service.Login(context.Background(), Input{Email: first.Email, Password: "correct horse battery", IP: &ip}); !errors.Is(err, ErrIPRateLimited) {
		t.Fatalf("same IP correct-password login error = %v, want rate limited", err)
	}
	otherIP := netip.MustParseAddr("198.51.100.11")
	if _, err := service.Login(context.Background(), Input{Email: first.Email, Password: "correct horse battery", IP: &otherIP}); err != nil {
		t.Fatalf("different IP login error = %v", err)
	}
}

func TestRF017_CuentaBloqueadaRegistraAuditoriaYPublicaEvento(t *testing.T) {
	now := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	user := activeUser(t, "ada@example.com")
	repository := &lockoutRepository{users: map[string]User{user.Email: user}}
	service := newLockoutService(t, repository, LockoutConfig{AccountMaxFailures: 1, IPMaxFailures: 20, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute})
	service.now = func() time.Time { return now }
	publisher := &stubSecurityEventPublisher{}
	service.WithEventPublisher(publisher)

	if _, err := service.Login(context.Background(), Input{Email: user.Email, Password: "wrong password"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("failed login error = %v", err)
	}

	var lockedAudits int
	for _, audit := range repository.audits {
		if audit.Action == "account_locked" {
			lockedAudits++
			if audit.ActorUserID == nil || *audit.ActorUserID != user.ID {
				t.Fatalf("account_locked audit actor = %v, want %v", audit.ActorUserID, user.ID)
			}
		}
	}
	if lockedAudits != 1 {
		t.Fatalf("account_locked audits = %d, want 1", lockedAudits)
	}
	if len(publisher.events) != 1 || publisher.events[0].Type != "security.account_locked" || publisher.events[0].UserID != user.ID {
		t.Fatalf("published events = %+v", publisher.events)
	}
}
