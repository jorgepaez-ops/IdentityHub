package login

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/config"
)

type repositoryStub struct {
	user      User
	lookupErr error
	roles     []string
	refresh   RefreshToken
	audits    []AuditEvent
	rehashed  []string
}

type testMFAIssuer struct{ issued *[]mfa.User }

func (i testMFAIssuer) Issue(_ context.Context, user mfa.User) (mfa.Challenge, error) {
	if i.issued != nil {
		*i.issued = append(*i.issued, user)
	}
	return mfa.Challenge{Token: "mfa-token", ExpiresIn: 300}, nil
}
func withMFA(repository Repository, signer *token.Service, ttl time.Duration) *Service {
	return New(repository, signer, ttl).WithMFA(testMFAIssuer{})
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
func (r *repositoryStub) UpdatePasswordHash(_ context.Context, _ uuid.UUID, hash string) error {
	r.rehashed = append(r.rehashed, hash)
	return nil
}
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
	service := withMFA(repository, signer, 720*time.Hour)

	result, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.MfaToken == "" || result.ExpiresIn != 300 {
		t.Fatalf("result = %+v", result)
	}
	if repository.refresh.FamilyID != uuid.Nil || len(repository.audits) != 0 {
		t.Fatalf("audits = %+v", repository.audits)
	}
	if len(repository.rehashed) != 0 {
		t.Fatalf("current-parameter hash was rewritten: %v", repository.rehashed)
	}
}

func TestRF013_CuentaConMFAIniciaDesafioSinTokens(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: StatusActive, MFAEnabled: true}}
	result, err := withMFA(repository, signer, time.Hour).Login(context.Background(), Input{Password: "correct horse battery"})
	if err != nil || result.MfaToken == "" || repository.refresh.FamilyID != uuid.Nil {
		t.Fatalf("result=%+v err=%v refresh=%+v", result, err, repository.refresh)
	}
	if len(repository.audits) != 0 {
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
	result, err := withMFA(repository, signer, time.Hour).Login(context.Background(), Input{Password: "correct horse battery"})
	if !errors.Is(err, ErrInvalidCredentials) || result.MfaToken != "" {
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
	_, err = withMFA(repository, signer, time.Hour).Login(context.Background(), Input{Password: "wrong password"})
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
	_, unknownErr := withMFA(unknown, signer, time.Hour).Login(context.Background(), Input{Email: "missing@example.com", Password: "wrong password"})
	_, wrongPasswordErr := withMFA(wrongPassword, signer, time.Hour).Login(context.Background(), Input{Email: "known@example.com", Password: "wrong password"})
	if !errors.Is(unknownErr, ErrInvalidCredentials) || !errors.Is(wrongPasswordErr, ErrInvalidCredentials) || unknownErr.Error() != wrongPasswordErr.Error() {
		t.Fatalf("unknown=%v wrong-password=%v", unknownErr, wrongPasswordErr)
	}
}

func TestRF009_TokenDeConsolaSoloContieneRolesDeDirectorio(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repository := &repositoryStub{
		user:  User{ID: id, Email: "senior@example.com", PasswordHash: hash, Status: StatusActive},
		roles: []string{"user", "contabilidad.senior"},
	}
	result, err := withMFA(repository, signer, time.Hour).Login(context.Background(), Input{Email: "senior@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.MfaToken == "" {
		t.Fatalf("result=%+v", result)
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
	service := withMFA(repository, signer, time.Hour)

	for attempt := 1; attempt <= 5; attempt++ {
		if _, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "wrong password"}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d error = %v, want invalid credentials", attempt, err)
		}
	}
	if _, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"}); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("sixth attempt error = %v, want account locked", err)
	}
}

func TestRF013_LoginPasaNombreYCorreoAlDesafioMFA(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repository := &repositoryStub{user: User{ID: id, Email: "ada@example.com", DisplayName: "Ada Lovelace", PasswordHash: hash, Status: StatusActive}}
	var issued []mfa.User
	service := New(repository, signer, time.Hour).WithMFA(testMFAIssuer{issued: &issued})
	if _, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"}); err != nil {
		t.Fatal(err)
	}
	if len(issued) != 1 || issued[0].ID != id || issued[0].Email != "ada@example.com" || issued[0].DisplayName != "Ada Lovelace" {
		t.Fatalf("issued = %+v", issued)
	}
}

func TestRF013_LoginSoloReescribeElHashObsoletoYNoRegistraExito(t *testing.T) {
	restore := password.Configure(config.PasswordConfig{MemoryKiB: 8, Iterations: 1, Parallelism: 1, Concurrency: 1})
	weakHash, err := password.Hash("correct horse battery")
	restore()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), Email: "ada@example.com", PasswordHash: weakHash, Status: StatusActive}}
	if _, err := withMFA(repository, signer, time.Hour).Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"}); err != nil {
		t.Fatal(err)
	}
	if len(repository.rehashed) != 1 || repository.rehashed[0] == weakHash {
		t.Fatalf("rehashed = %v", repository.rehashed)
	}
	if len(repository.audits) != 0 {
		t.Fatalf("password step must not audit login success (deferred to MFA): %+v", repository.audits)
	}
}
