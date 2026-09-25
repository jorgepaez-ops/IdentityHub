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

type repositoryStub struct {
	user      User
	lookupErr error
	roles     []string
	refresh   RefreshToken
	audits    []AuditEvent
}

func (r *repositoryStub) WithinLoginTransaction(_ context.Context, fn func(Writer) error) error {
	return fn(r)
}
func (r *repositoryStub) GetLoginUserByEmail(context.Context, string) (User, error) {
	return r.user, r.lookupErr
}
func (r *repositoryStub) CountLoginFailuresByAccount(_ context.Context, userID uuid.UUID, _ time.Time) (int64, error) {
	var count int64
	for _, audit := range r.audits {
		if audit.Action == "login_failed" && audit.ActorUserID != nil && *audit.ActorUserID == userID {
			count++
		}
	}
	return count, nil
}
func (r *repositoryStub) CountLoginFailuresByIP(_ context.Context, ip netip.Addr, _ time.Time) (int64, error) {
	var count int64
	for _, audit := range r.audits {
		if audit.Action == "login_failed" && audit.IP != nil && *audit.IP == ip {
			count++
		}
	}
	return count, nil
}
func (r *repositoryStub) LockLoginUser(_ context.Context, userID uuid.UUID, until time.Time) error {
	if r.user.ID == userID {
		r.user.Status = StatusLocked
		r.user.LockedUntil = &until
	}
	return nil
}
func (r *repositoryStub) UnlockLoginUser(_ context.Context, userID uuid.UUID) error {
	if r.user.ID == userID {
		r.user.Status = StatusActive
		r.user.LockedUntil = nil
	}
	return nil
}
func (r *repositoryStub) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	return r.roles, nil
}
func (r *repositoryStub) CreateRefreshToken(_ context.Context, value RefreshToken) error {
	r.refresh = value
	return nil
}
func (r *repositoryStub) UpdateLoginSuccess(context.Context, uuid.UUID, string) error { return nil }
func (r *repositoryStub) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

func TestRF003_LoginCorrectoEmiteTokensYGuardaRefresh(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repository := &repositoryStub{user: User{ID: id, Email: "ada@example.com", PasswordHash: hash, Status: StatusActive}, roles: []string{"user"}}
	service := New(repository, signer, 720*time.Hour)

	result, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.AccessToken == "" || result.ExpiresIn != 900 || result.TokenType != "Bearer" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.RefreshToken) == 0 || len(repository.refresh.TokenHash) != 32 || repository.refresh.FamilyID == uuid.Nil {
		t.Fatalf("refresh was not securely persisted: %+v", repository.refresh)
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "login_succeeded" {
		t.Fatalf("audits = %+v", repository.audits)
	}
}

func TestRF003_CuentaConMFARechazadaSinTokens(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: StatusActive, MFAEnabled: true}}
	result, err := New(repository, signer, time.Hour).Login(context.Background(), Input{Password: "correct horse battery"})
	if !errors.Is(err, ErrMFAUnavailable) || result.RefreshToken != "" || repository.refresh.FamilyID != uuid.Nil {
		t.Fatalf("result=%+v err=%v refresh=%+v", result, err, repository.refresh)
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "login_failed" || repository.audits[0].Reason != "mfa_not_supported" {
		t.Fatalf("audits=%+v", repository.audits)
	}
}

func TestRF003_CuentaSinVerificarNoEntra(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: "pending_verification"}}
	result, err := New(repository, signer, time.Hour).Login(context.Background(), Input{Password: "correct horse battery"})
	if !errors.Is(err, ErrInvalidCredentials) || result.AccessToken != "" || result.RefreshToken != "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "login_failed" {
		t.Fatalf("audits=%+v", repository.audits)
	}
}

func TestRF003_LoginFallidoQuedaEnAuditoria(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repository := &repositoryStub{user: User{ID: id, PasswordHash: hash, Status: StatusActive}}
	_, err = New(repository, signer, time.Hour).Login(context.Background(), Input{Password: "wrong password"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want invalid credentials", err)
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "login_failed" || repository.audits[0].ActorUserID == nil || *repository.audits[0].ActorUserID != id {
		t.Fatalf("audits=%+v", repository.audits)
	}
}

func TestRF003_AM004EmailInexistenteYPasswordIncorrectoCompartenError(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	unknown := &repositoryStub{lookupErr: pgx.ErrNoRows}
	wrongPassword := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: StatusActive}}
	_, unknownErr := New(unknown, signer, time.Hour).Login(context.Background(), Input{Email: "missing@example.com", Password: "wrong password"})
	_, wrongPasswordErr := New(wrongPassword, signer, time.Hour).Login(context.Background(), Input{Email: "known@example.com", Password: "wrong password"})
	if !errors.Is(unknownErr, ErrInvalidCredentials) || !errors.Is(wrongPasswordErr, ErrInvalidCredentials) || unknownErr.Error() != wrongPasswordErr.Error() {
		t.Fatalf("unknown=%v wrong-password=%v", unknownErr, wrongPasswordErr)
	}
}

func TestRF017_SeisFallosDevuelven423(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), Email: "ada@example.com", PasswordHash: hash, Status: StatusActive}, roles: []string{"user"}}
	service := New(repository, signer, time.Hour)

	for attempt := 1; attempt <= 5; attempt++ {
		if _, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "wrong password"}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d error = %v, want invalid credentials", attempt, err)
		}
	}
	if _, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"}); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("sixth attempt error = %v, want account locked", err)
	}
}
