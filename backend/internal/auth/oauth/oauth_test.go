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
	"github.com/jorgepaez/identity-hub/internal/auth/roles"
)

var testOrganization = uuid.MustParse("00000000-0000-4000-8000-0000000000a2")

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
			service := New(&memoryRepository{}, []Client{client}, bytesReader(), func() time.Time { return now })
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
	service := New(repository, []Client{client}, bytesReader(), func() time.Time { return now })
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
		service := New(repository, []Client{client}, bytesReader(), func() time.Time { return clock })
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
	service := New(repository, []Client{client}, bytesReader(), func() time.Time { return now })
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
		service := New(repository, []Client{client}, bytesReader(), func() time.Time { return now })
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
	service := New(repository, []Client{client}, bytesReader(), time.Now)
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
	service := New(repository, []Client{client}, bytesReader(), time.Now)
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

// A failed permissions lookup must abort the transaction: no result (so no token
// is issued) and the code stays redeemable, as with any other post-consumption failure.
func TestRF020_FalloDeLaConsultaDePermisosAbortaElCanjeYNoEmiteToken(t *testing.T) {
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{roles: []string{"user", "contabilidad.senior"}, permissions: []string{"reportes.ver"}, permissionsErr: errors.New("permissions unavailable")}
	service := New(repository, []Client{client}, bytesReader(), time.Now)
	issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	input := ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}
	result, err := service.Exchange(context.Background(), input)
	if !errors.Is(err, repository.permissionsErr) {
		t.Fatalf("exchange error=%v, want wrapped permissions failure", err)
	}
	if result.UserID != uuid.Nil || len(result.Roles) != 0 || len(result.Permissions) != 0 {
		t.Fatalf("result=%+v, want empty so that no token is issued", result)
	}
	repository.permissionsErr = nil
	if _, err := service.Exchange(context.Background(), input); err != nil {
		t.Fatalf("exchange after rolled-back permissions failure: %v", err)
	}
}

func TestRF020_FalloDeAuditoriaRevierteElCodigo(t *testing.T) {
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	repository := &memoryRepository{roles: []string{"user"}}
	service := New(repository, []Client{client}, bytesReader(), time.Now)
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

func TestRF020_ErrorDeRepositorioEnCanjeEsInternoYNoAudita(t *testing.T) {
	client := Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/callback"}
	outage := errors.New("database unavailable")
	repository := &memoryRepository{roles: []string{"user"}, exchangeErr: outage}
	service := New(repository, []Client{client}, bytesReader(), time.Now)
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
	service := New(repository, []Client{client}, bytesReader(), time.Now)
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
	access              *AuthorizationAccess
	audits              []AuditEvent
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

// ListApplicationRolesForUser emulates the real query (roles.application_id)
// by the "<client_id>." name prefix the seeded catalog follows.
func (m *memoryRepository) ListApplicationRolesForUser(_ context.Context, _ uuid.UUID, clientID string) ([]string, error) {
	if m.rolesErr != nil {
		return nil, m.rolesErr
	}
	return roles.ForApplication(m.roles, clientID), nil
}

func (m *memoryRepository) GetAuthorizationAccess(_ context.Context, _ uuid.UUID, _ string) (AuthorizationAccess, error) {
	if m.access != nil {
		return *m.access, nil
	}
	return AuthorizationAccess{UserOrganizationID: testOrganization, ApplicationOrganizationID: testOrganization, HasApplicationRole: true}, nil
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
	m.audits = append(m.audits, event)
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

func inventarioClient() Client {
	return Client{ID: "inventario", RedirectURI: "http://inventario.localhost:8080/oauth/callback", Origin: "http://inventario.localhost:8080", OrganizationID: uuid.MustParse("00000000-0000-4000-8000-0000000000a3")}
}

func contabilidadClient() Client {
	return Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080", OrganizationID: testOrganization}
}

func TestRF020_VariosClientesRegistradosCadaUnoConSuRedirectExacto(t *testing.T) {
	service := New(&memoryRepository{}, []Client{contabilidadClient(), inventarioClient()}, bytesReader(), time.Now)
	for _, client := range []Client{contabilidadClient(), inventarioClient()} {
		if got, ok := service.TrustedClient(validAuthorizeInput(client)); !ok || got.ID != client.ID {
			t.Fatalf("TrustedClient(%s)=%+v,%v", client.ID, got, ok)
		}
	}
	crossed := validAuthorizeInput(inventarioClient())
	crossed.RedirectURI = contabilidadClient().RedirectURI
	if _, ok := service.TrustedClient(crossed); ok {
		t.Fatal("inventario was trusted with the redirect URI of contabilidad")
	}
	unknown := validAuthorizeInput(inventarioClient())
	unknown.ClientID = "desconocido"
	if _, err := service.Authorize(context.Background(), uuid.New(), unknown); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("unknown client error=%v, want ErrInvalidClient", err)
	}
}

func TestRF020_OrigenPermitidoEsElDeCualquierClienteRegistrado(t *testing.T) {
	service := New(&memoryRepository{}, []Client{contabilidadClient(), inventarioClient()}, bytesReader(), time.Now)
	for origin, want := range map[string]bool{
		"http://contabilidad.localhost:8080":    true,
		"http://inventario.localhost:8080":      true,
		"http://inventario.localhost:8080/":     false,
		"http://evil.example":                   false,
		"HTTP://INVENTARIO.LOCALHOST:8080":      false,
		"":                                      false,
		"null":                                  false,
		"http://inventario.localhost:8080.evil": false,
	} {
		if got := service.AllowsOrigin(origin); got != want {
			t.Errorf("AllowsOrigin(%q)=%v, want %v", origin, got, want)
		}
	}
}

func TestRF022_AutorizacionRechazadaNoEmiteCodigoYAudita(t *testing.T) {
	user := uuid.New()
	for _, testCase := range []struct {
		name       string
		access     AuthorizationAccess
		wantReason string
	}{
		{"empresa distinta", AuthorizationAccess{UserOrganizationID: testOrganization, ApplicationOrganizationID: inventarioClient().OrganizationID, HasApplicationRole: false}, RejectionOrganizationMismatch},
		{"empresa distinta aunque tenga rol", AuthorizationAccess{UserOrganizationID: testOrganization, ApplicationOrganizationID: inventarioClient().OrganizationID, HasApplicationRole: true}, RejectionOrganizationMismatch},
		{"misma empresa sin rol de la aplicación", AuthorizationAccess{UserOrganizationID: inventarioClient().OrganizationID, ApplicationOrganizationID: inventarioClient().OrganizationID, HasApplicationRole: false}, RejectionNoApplicationRole},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repository := &memoryRepository{access: &testCase.access}
			service := New(repository, []Client{inventarioClient()}, bytesReader(), time.Now)
			result, err := service.Authorize(context.Background(), user, validAuthorizeInput(inventarioClient()))
			var rejected *RejectedError
			if !errors.As(err, &rejected) || rejected.Reason != testCase.wantReason {
				t.Fatalf("error=%v, want RejectedError with reason %q", err, testCase.wantReason)
			}
			if result.Code != "" || len(repository.code.CodeHash) != 0 {
				t.Fatalf("a code was issued: result=%+v stored=%+v", result, repository.code)
			}
			if len(repository.audits) != 1 {
				t.Fatalf("audits=%+v, want exactly one", repository.audits)
			}
			event := repository.audits[0]
			if event.Action != "oauth_authorization_rejected" || event.ActorUserID != user || event.Metadata["reason"] != testCase.wantReason || event.Metadata["client_id"] != "inventario" {
				t.Fatalf("audit=%+v", event)
			}
		})
	}
}

func TestRF022_FalloDeAuditoriaDelRechazoNoEmiteCodigo(t *testing.T) {
	repository := &memoryRepository{access: &AuthorizationAccess{UserOrganizationID: testOrganization, ApplicationOrganizationID: inventarioClient().OrganizationID}, auditErr: errors.New("audit unavailable")}
	service := New(repository, []Client{inventarioClient()}, bytesReader(), time.Now)
	_, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(inventarioClient()))
	var rejected *RejectedError
	if err == nil || errors.As(err, &rejected) || len(repository.code.CodeHash) != 0 {
		t.Fatalf("error=%v stored=%+v, want an internal error and no code", err, repository.code)
	}
}

func TestRF022_CanjeDeInventarioLlevaSoloSusRolesYSuAudiencia(t *testing.T) {
	repository := &memoryRepository{
		roles:       []string{"user", "contabilidad.senior", "inventario.bodeguero", "inventario.consulta"},
		permissions: []string{"movimientos.registrar", "productos.ver"},
	}
	service := New(repository, []Client{contabilidadClient(), inventarioClient()}, bytesReader(), time.Now)
	client := inventarioClient()
	issued, err := service.Authorize(context.Background(), uuid.New(), validAuthorizeInput(client))
	if err != nil {
		t.Fatal(err)
	}
	if repository.code.ClientID != "inventario" || repository.code.RedirectURI != client.RedirectURI {
		t.Fatalf("stored code=%+v, want it bound to inventario", repository.code)
	}
	result, err := service.Exchange(context.Background(), ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ClientID != "inventario" {
		t.Fatalf("ClientID=%q, want the audience inventario", result.ClientID)
	}
	if got, want := result.Roles, []string{"inventario.bodeguero", "inventario.consulta"}; !sameStrings(got, want) {
		t.Fatalf("roles=%v, want %v", got, want)
	}
	if repository.permissionClientID != "inventario" {
		t.Fatalf("permissions looked up for %q", repository.permissionClientID)
	}
}

func TestRF020_ValidateClientsExigeAlMenosUnClienteBienFormado(t *testing.T) {
	valid := contabilidadClient()
	for name, testCase := range map[string]struct {
		clients []Client
		wantErr bool
	}{
		"dos registros válidos":  {[]Client{valid, inventarioClient()}, false},
		"sin registros":          {nil, true},
		"id con espacios":        {[]Client{{ID: "mi cliente", RedirectURI: valid.RedirectURI, Origin: valid.Origin}}, true},
		"redirect relativo":      {[]Client{{ID: "a", RedirectURI: "/oauth/callback", Origin: valid.Origin}}, true},
		"redirect con fragmento": {[]Client{{ID: "a", RedirectURI: valid.RedirectURI + "#x", Origin: valid.Origin}}, true},
		"esquema no http":        {[]Client{{ID: "a", RedirectURI: "ftp://x.test/cb", Origin: "ftp://x.test"}}, true},
		"origen con ruta":        {[]Client{{ID: "a", RedirectURI: valid.RedirectURI, Origin: valid.Origin + "/app"}}, true},
		"origen no coincide":     {[]Client{{ID: "a", RedirectURI: valid.RedirectURI, Origin: "https://otro.example.test"}}, true},
		"client id repetido":     {[]Client{valid, valid}, true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateClients(testCase.clients); (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateClients error=%v, wantErr=%v", err, testCase.wantErr)
			}
		})
	}
}
