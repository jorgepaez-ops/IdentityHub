package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRF020_ControlesDeAutorizacionPKCE(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback", Origin: "http://contabilidad.localhost:8080"}
	userID := uuid.New()
	for _, testCase := range []struct {
		name  string
		input AuthorizeInput
		want  error
	}{
		{"exact redirect URI", AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI + "/other", CodeChallenge: challenge("verifier")}, ErrInvalidClient},
		{"state mandatory", AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, CodeChallenge: challenge("verifier")}, ErrInvalidRequest},
		{"S256 only", AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, State: "state", CodeChallenge: challenge("verifier"), CodeChallengeMethod: "plain"}, ErrInvalidRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := New(&memoryRepository{}, client, bytesReader(), func() time.Time { return now })
			_, err := service.Authorize(context.Background(), userID, testCase.input)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("Authorize error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestRF020_CodigoEsDeUnUsoYVencePronto(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{roles: []string{"admin", "user", "contabilidad.senior"}}
	service := New(repository, client, bytesReader(), func() time.Time { return now })
	owner := uuid.New()
	issued, err := service.Authorize(context.Background(), owner, validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	repository.auditAction = ""
	if issued.ExpiresAt.Sub(now) != AuthorizationCodeTTL {
		t.Fatalf("expiry = %v", issued.ExpiresAt.Sub(now))
	}
	result, err := service.Exchange(context.Background(), ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "contabilidad.senior" {
		t.Fatalf("roles=%v", result.Roles)
	}
	if _, err := service.Exchange(context.Background(), ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}); !errors.Is(err, ErrAuthorizationCodeInvalid) {
		t.Fatalf("reuse error=%v", err)
	}
	if repository.auditAction != "authorization_code_reused" || repository.auditActor != owner {
		t.Fatalf("audit=%q actor=%s want owner %s", repository.auditAction, repository.auditActor, owner)
	}
	t.Run("expired code is rejected", func(t *testing.T) {
		clock := now
		repository := &memoryRepository{roles: []string{"user"}}
		service := New(repository, client, bytesReader(), func() time.Time { return clock })
		issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
		if err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(AuthorizationCodeTTL + time.Second)
		repository.auditAction = ""
		if _, err := service.Exchange(context.Background(), ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}); !errors.Is(err, ErrAuthorizationCodeInvalid) {
			t.Fatalf("expired exchange error=%v", err)
		}
		if repository.auditAction != "" {
			t.Fatalf("expired exchange wrote audit %q", repository.auditAction)
		}
	})
}

func TestRF020_CanjeResuelveLaUnionDePermisosDeLaAplicacion(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{
		roles:       []string{"user", "contabilidad.senior", "contabilidad.analista", "otra.operador"},
		permissions: []string{"movimientos.registrar", "movimientos.ver_todos", "reportes.ver"},
	}
	service := New(repository, client, bytesReader(), func() time.Time { return now })
	issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Exchange(context.Background(), ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Roles, []string{"contabilidad.senior", "contabilidad.analista"}; !sameStrings(got, want) {
		t.Fatalf("roles=%v, want %v", got, want)
	}
	if got, want := result.Permissions, []string{"movimientos.registrar", "movimientos.ver_todos", "reportes.ver"}; !sameStrings(got, want) {
		t.Fatalf("permissions=%v, want %v", got, want)
	}
	if got := repository.permissionRoleNames; !sameStrings(got, result.Roles) || repository.permissionClientID != client.ID {
		t.Fatalf("permission lookup roles=%v client=%q", got, repository.permissionClientID)
	}
}

func TestRF020_CanjeRechazaVerificadorClienteYRedirectDistintos(t *testing.T) {
	now := time.Now().UTC()
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	for _, exchange := range []ExchangeInput{
		{ClientID: "other", RedirectURI: client.RedirectURI, CodeVerifier: "verifier"},
		{ClientID: client.ID, RedirectURI: client.RedirectURI + "/other", CodeVerifier: "verifier"},
		{ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "wrong"},
	} {
		repository := &memoryRepository{roles: []string{"user"}}
		service := New(repository, client, bytesReader(), func() time.Time { return now })
		issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
		if err != nil {
			t.Fatal(err)
		}
		exchange.Code = issued.Code
		repository.auditAction = ""
		if _, err := service.Exchange(context.Background(), exchange); !errors.Is(err, ErrAuthorizationCodeInvalid) {
			t.Fatalf("exchange=%+v error=%v", exchange, err)
		}
		if repository.auditAction != "" {
			t.Fatalf("exchange=%+v wrote audit %q", exchange, repository.auditAction)
		}
	}
}

func TestRF020_VerificadorIncorrectoNoConsumeCodigo(t *testing.T) {
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{roles: []string{"user"}}
	service := New(repository, client, bytesReader(), time.Now)
	issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	wrong := ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "wrong"}
	if _, err := service.Exchange(context.Background(), wrong); !errors.Is(err, ErrAuthorizationCodeInvalid) {
		t.Fatalf("wrong verifier error=%v", err)
	}
	if _, err := service.Exchange(context.Background(), ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}); err != nil {
		t.Fatalf("correct verifier after rejected one: %v", err)
	}
}

func TestRF020_FalloPosteriorAlConsumoRevierteElCodigo(t *testing.T) {

	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{roles: []string{"user"}, rolesErr: errors.New("roles unavailable")}
	service := New(repository, client, bytesReader(), time.Now)
	issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	input := ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}
	if _, err := service.Exchange(context.Background(), input); !errors.Is(err, repository.rolesErr) {
		t.Fatalf("first exchange error=%v, want wrapped role failure", err)
	}
	repository.rolesErr = nil
	if _, err := service.Exchange(context.Background(), input); err != nil {
		t.Fatalf("exchange after rolled-back failure: %v", err)
	}
}

func TestRF020_FalloDeAuditoriaRevierteElCodigo(t *testing.T) {
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{roles: []string{"user"}}
	service := New(repository, client, bytesReader(), time.Now)
	issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	repository.auditErr = errors.New("audit unavailable")
	input := ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}
	if _, err := service.Exchange(context.Background(), input); !errors.Is(err, repository.auditErr) {
		t.Fatalf("first exchange error=%v, want wrapped audit failure", err)
	}
	repository.auditErr = nil
	if _, err := service.Exchange(context.Background(), input); err != nil {
		t.Fatalf("exchange after rolled-back audit failure: %v", err)
	}
}

func TestRF020_RegistroDelClienteDebeCoincidirConLaConfiguracion(t *testing.T) {
	configured := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}
	registered := configured
	if err := ValidateRegisteredClient(configured, registered); err != nil {
		t.Fatalf("matching registered client: %v", err)
	}
	registered.RedirectURI = "http://contabilidad.localhost:8080/other"
	if err := ValidateRegisteredClient(configured, registered); err == nil {
		t.Fatal("mismatched client registration was accepted")
	}
}

func TestRF020_ErrorDeRepositorioEnCanjeEsInternoYNoAudita(t *testing.T) {
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	outage := errors.New("database unavailable")
	repository := &memoryRepository{roles: []string{"user"}, exchangeErr: outage}
	service := New(repository, client, bytesReader(), time.Now)
	issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	repository.auditAction = ""
	_, err = service.Exchange(context.Background(), ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"})
	if errors.Is(err, ErrAuthorizationCodeInvalid) || !errors.Is(err, outage) {
		t.Fatalf("error=%v, want wrapped outage and not invalid grant", err)
	}
	if repository.auditAction != "" {
		t.Fatalf("outage wrote audit %q", repository.auditAction)
	}
}

func TestRF020_CodigoDesconocidoNoAudita(t *testing.T) {
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{roles: []string{"user"}}
	service := New(repository, client, bytesReader(), time.Now)
	unknown := base64.RawURLEncoding.EncodeToString([]byte("never-issued-code-bytes"))
	if _, err := service.Exchange(context.Background(), ExchangeInput{Code: unknown, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}); !errors.Is(err, ErrAuthorizationCodeInvalid) {
		t.Fatalf("error=%v", err)
	}
	if repository.auditAction != "" {
		t.Fatalf("unknown code wrote audit %q", repository.auditAction)
	}
}

func validAuthorizeInput(client Client) AuthorizeInput {
	return AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, ResponseType: "code", State: "state", CodeChallenge: challenge("verifier"), CodeChallengeMethod: "S256"}
}
func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func bytesReader() io.Reader { return &repeatReader{} }

type repeatReader struct{}

func (*repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(i + 1)
	}
	return len(p), nil
}

type memoryRepository struct {
	code                StoredCode
	roles               []string
	auditAction         string
	auditActor          uuid.UUID
	exchangeErr         error
	rolesErr            error
	permissions         []string
	permissionsErr      error
	permissionRoleNames []string
	permissionClientID  string
	auditErr            error
	inTx                bool
}

func (m *memoryRepository) WithinAuthorizationCodeTransaction(_ context.Context, fn func(ExchangeWriter) error) error {
	before := m.code
	m.inTx = true
	err := fn(m)
	m.inTx = false
	if err != nil {
		m.code = before
	}
	return err
}

func (m *memoryRepository) CreateAuthorizationCode(_ context.Context, code CreateCode) error {
	m.code = StoredCode{ID: code.ID, UserID: code.UserID, ClientID: code.ClientID, RedirectURI: code.RedirectURI, CodeHash: append([]byte(nil), code.CodeHash...), CodeChallenge: code.CodeChallenge, ExpiresAt: code.ExpiresAt}
	return nil
}
func (m *memoryRepository) ExchangeAuthorizationCode(_ context.Context, hash []byte, clientID, redirectURI, challenge string, now time.Time) (StoredCode, error) {
	if m.exchangeErr != nil {
		return StoredCode{}, m.exchangeErr
	}
	if string(m.code.CodeHash) == string(hash) && !m.code.UsedAt.IsZero() {
		return StoredCode{UserID: m.code.UserID}, ErrAuthorizationCodeReused
	}
	if string(m.code.CodeHash) != string(hash) || !m.code.UsedAt.IsZero() || !now.Before(m.code.ExpiresAt) || m.code.ClientID != clientID || m.code.RedirectURI != redirectURI || subtle.ConstantTimeCompare([]byte(m.code.CodeChallenge), []byte(challenge)) != 1 {
		return StoredCode{}, ErrAuthorizationCodeInvalid
	}
	m.code.UsedAt = now
	return m.code, nil
}
func (m *memoryRepository) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	if m.rolesErr != nil {
		return nil, m.rolesErr
	}
	return m.roles, nil
}
func (m *memoryRepository) ListPermissionKeysForRolesAndApplication(_ context.Context, roleNames []string, clientID string) ([]string, error) {
	m.permissionRoleNames = append([]string(nil), roleNames...)
	m.permissionClientID = clientID
	if m.permissionsErr != nil {
		return nil, m.permissionsErr
	}
	return append([]string(nil), m.permissions...), nil
}
func (m *memoryRepository) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	if m.auditErr != nil {
		return m.auditErr
	}
	m.auditAction, m.auditActor = event.Action, event.ActorUserID
	return nil
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
