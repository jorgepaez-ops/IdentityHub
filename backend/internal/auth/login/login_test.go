package login

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/config"
)

type repositoryStub struct {
	user      User
	lookupErr error
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
func withMFA(repository Repository) *Service {
	return New(repository).WithMFA(testMFAIssuer{})
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
func (r *repositoryStub) UpdatePasswordHash(_ context.Context, _ uuid.UUID, hash string) error {
	r.rehashed = append(r.rehashed, hash)
	return nil
}
func (r *repositoryStub) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

func TestRF003_LoginCorrectoDevuelveDesafioMFASinSesion(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repository := &repositoryStub{user: User{ID: id, Email: "ada@example.com", PasswordHash: hash, Status: StatusActive}}
	service := withMFA(repository)

	result, err := service.Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.MfaToken == "" || result.ExpiresIn != 300 {
		t.Fatalf("result = %+v", result)
	}
	if len(repository.audits) != 0 {
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
	repository := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: StatusActive}}
	result, err := withMFA(repository).Login(context.Background(), Input{Password: "correct horse battery"})
	if err != nil || result.MfaToken == "" {
		t.Fatalf("result=%+v err=%v", result, err)
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
	repository := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: "pending_verification"}}
	result, err := withMFA(repository).Login(context.Background(), Input{Password: "correct horse battery"})
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
	id := uuid.New()
	repository := &repositoryStub{user: User{ID: id, PasswordHash: hash, Status: StatusActive}}
	_, err = withMFA(repository).Login(context.Background(), Input{Password: "wrong password"})
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
	unknown := &repositoryStub{lookupErr: pgx.ErrNoRows}
	wrongPassword := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: StatusActive}}
	_, unknownErr := withMFA(unknown).Login(context.Background(), Input{Email: "missing@example.com", Password: "wrong password"})
	_, wrongPasswordErr := withMFA(wrongPassword).Login(context.Background(), Input{Email: "known@example.com", Password: "wrong password"})
	if !errors.Is(unknownErr, ErrInvalidCredentials) || !errors.Is(wrongPasswordErr, ErrInvalidCredentials) || unknownErr.Error() != wrongPasswordErr.Error() {
		t.Fatalf("unknown=%v wrong-password=%v", unknownErr, wrongPasswordErr)
	}
}

func TestRF003_ContrasenaLargaDevuelveCredencialesInvalidasYSeAudita(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	cases := []struct {
		name       string
		repository *repositoryStub
		actorID    *uuid.UUID
	}{
		{
			name:       "known email",
			repository: &repositoryStub{user: User{ID: id, PasswordHash: hash, Status: StatusActive}},
			actorID:    &id,
		},
		{
			name:       "unknown email",
			repository: &repositoryStub{lookupErr: pgx.ErrNoRows},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := withMFA(testCase.repository).Login(context.Background(), Input{Email: "ada@example.com", Password: strings.Repeat("a", 129)})
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want invalid credentials", err)
			}
			if len(testCase.repository.audits) != 1 || testCase.repository.audits[0].Action != "login_failed" {
				t.Fatalf("audits=%+v", testCase.repository.audits)
			}
			gotActorID := testCase.repository.audits[0].ActorUserID
			if testCase.actorID == nil && gotActorID != nil {
				t.Fatalf("actor ID = %v, want nil", gotActorID)
			}
			if testCase.actorID != nil && (gotActorID == nil || *gotActorID != *testCase.actorID) {
				t.Fatalf("actor ID = %v, want %v", gotActorID, testCase.actorID)
			}
		})
	}
}

func TestRF017_ContrasenaLargaCuentaParaBloqueo(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: StatusActive}}
	service := withMFA(repository)
	for attempt := 1; attempt <= 5; attempt++ {
		if _, err := service.Login(context.Background(), Input{Password: strings.Repeat("a", 129)}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d error = %v, want invalid credentials", attempt, err)
		}
	}
	if _, err := service.Login(context.Background(), Input{Password: "correct horse battery"}); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("login after long-password failures error = %v, want account locked", err)
	}
	if repository.user.Status != StatusLocked {
		t.Fatalf("user status = %q, want %q", repository.user.Status, StatusLocked)
	}
}

func TestRF003_ContrasenaDe128CaracteresSeVerifica(t *testing.T) {
	passwordValue := strings.Repeat("a", 128)
	hash, err := password.Hash(passwordValue)
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), PasswordHash: hash, Status: StatusActive}}
	result, err := withMFA(repository).Login(context.Background(), Input{Password: passwordValue})
	if err != nil || result.MfaToken == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRF017_SeisFallosDevuelven423(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{user: User{ID: uuid.New(), Email: "ada@example.com", PasswordHash: hash, Status: StatusActive}}
	service := withMFA(repository)

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
	id := uuid.New()
	repository := &repositoryStub{user: User{ID: id, Email: "ada@example.com", DisplayName: "Ada Lovelace", PasswordHash: hash, Status: StatusActive}}
	var issued []mfa.User
	service := New(repository).WithMFA(testMFAIssuer{issued: &issued})
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
	repository := &repositoryStub{user: User{ID: uuid.New(), Email: "ada@example.com", PasswordHash: weakHash, Status: StatusActive}}
	if _, err := withMFA(repository).Login(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery"}); err != nil {
		t.Fatal(err)
	}
	if len(repository.rehashed) != 1 || repository.rehashed[0] == weakHash {
		t.Fatalf("rehashed = %v", repository.rehashed)
	}
	if len(repository.audits) != 0 {
		t.Fatalf("password step must not audit login success (deferred to MFA): %+v", repository.audits)
	}
}
