package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/auth/servicekey"
)

type serviceKeyStub struct {
	created      servicekey.Created
	keys         []servicekey.Key
	err          error
	authErr      error
	createInput  servicekey.CreateInput
	revokedID    uuid.UUID
	revokeInput  servicekey.RevokeInput
	authCalls    int
	authRaw      string
	authScope    string
	authMeta     servicekey.RequestMeta
	listCalls    int
	createCalled int
}

func (s *serviceKeyStub) Create(_ context.Context, input servicekey.CreateInput) (servicekey.Created, error) {
	s.createCalled++
	s.createInput = input
	return s.created, s.err
}
func (s *serviceKeyStub) List(context.Context) ([]servicekey.Key, error) {
	s.listCalls++
	return s.keys, s.err
}
func (s *serviceKeyStub) Revoke(_ context.Context, id uuid.UUID, input servicekey.RevokeInput) error {
	s.revokedID, s.revokeInput = id, input
	return s.err
}
func (s *serviceKeyStub) Authenticate(_ context.Context, raw, scope string, meta servicekey.RequestMeta) (servicekey.Principal, error) {
	s.authCalls++
	s.authRaw, s.authScope, s.authMeta = raw, scope, meta
	if s.authErr != nil {
		return servicekey.Principal{}, s.authErr
	}
	return servicekey.Principal{KeyID: uuid.New(), PublicID: "0123456789abcdef", Scopes: []string{servicekey.ScopeAuditLogRead}}, nil
}

const sampleBearer = "ihk_0123456789abcdef_0123456789012345678901234567890123456789012"

func serviceKeyServer(t *testing.T, actor uuid.UUID, roles []string, stub *serviceKeyStub) (*Server, *auditLogServiceStub) {
	t.Helper()
	logs := &auditLogServiceStub{events: []auditlog.Event{{ID: 1, Action: "login_failed", Metadata: map[string]any{}, CreatedAt: time.Now()}}}
	server := auditLogServer(t, actor, roles, logs)
	server.SetServiceKeyService(stub)
	return server, logs
}

func serviceKeyBearer(server *Server, method, path, body, key string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("User-Agent", "siem-tests")
	request.RemoteAddr = "203.0.113.20:5000"
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	return response
}

func TestRF018_AdminEmiteUnaClaveYLaRespuestaLaMuestraUnaVez(t *testing.T) {
	actor := uuid.New()
	id := uuid.New()
	stub := &serviceKeyStub{created: servicekey.Created{
		Key:     servicekey.Key{ID: id, PublicID: "0123456789abcdef", Name: "SIEM", Scopes: []string{servicekey.ScopeAuditLogRead}, CreatedBy: &actor, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
		FullKey: sampleBearer,
	}}
	server, _ := serviceKeyServer(t, actor, []string{"admin"}, stub)

	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/service-keys", `{"name":"SIEM","scopes":["audit-log:read"],"expiresInDays":30}`, actor)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body CreatedServiceKey
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Key != sampleBearer || body.Prefix != "ihk_0123456789abcdef" || body.Id != id || body.Name != "SIEM" {
		t.Fatalf("body=%+v", body)
	}
	if stub.createInput.ActorUserID != actor || stub.createInput.ExpiresInDays != 30 || stub.createInput.Name != "SIEM" || len(stub.createInput.Scopes) != 1 {
		t.Fatalf("input=%+v", stub.createInput)
	}
	if !stub.createInput.IP.IsValid() {
		t.Fatalf("request context was not recorded: %+v", stub.createInput)
	}
}

func TestRF018_EmisionRechazaPeticionesInvalidas(t *testing.T) {
	actor := uuid.New()
	for name, body := range map[string]string{
		"bad json":      `{`,
		"unknown field": `{"name":"x","scopes":["audit-log:read"],"expiresInDays":3,"admin":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			stub := &serviceKeyStub{}
			server, _ := serviceKeyServer(t, actor, []string{"admin"}, stub)
			response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/service-keys", body, actor)
			if response.Code != http.StatusBadRequest || stub.createCalled != 0 {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, stub.createCalled, response.Body.String())
			}
		})
	}
	stub := &serviceKeyStub{err: &servicekey.InputError{Field: "expiresInDays", Detail: "must be between 1 and 365"}}
	server, _ := serviceKeyServer(t, actor, []string{"admin"}, stub)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/service-keys", `{"name":"x","scopes":["audit-log:read"],"expiresInDays":9999}`, actor)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "expiresInDays") || !strings.Contains(response.Header().Get("Content-Type"), "problem+json") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF018_ListadoDevuelveSoloMetadatos(t *testing.T) {
	actor := uuid.New()
	used := time.Now()
	stub := &serviceKeyStub{keys: []servicekey.Key{{ID: uuid.New(), PublicID: "0123456789abcdef", Name: "SIEM", Scopes: []string{servicekey.ScopeAuditLogRead}, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), LastUsedAt: &used}}}
	server, _ := serviceKeyServer(t, actor, []string{"admin"}, stub)

	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/service-keys", "", actor)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var raw []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil || len(raw) != 1 {
		t.Fatalf("body=%s err=%v", response.Body.String(), err)
	}
	for field := range raw[0] {
		switch field {
		case "id", "name", "scopes", "prefix", "createdAt", "createdBy", "expiresAt", "lastUsedAt", "revokedAt":
		default:
			t.Fatalf("unexpected field %q in key metadata", field)
		}
	}
	if raw[0]["prefix"] != "ihk_0123456789abcdef" {
		t.Fatalf("prefix=%v", raw[0]["prefix"])
	}
}

func TestRF018_RevocaEsIdempotenteYDevuelve404SiNoExiste(t *testing.T) {
	actor, id := uuid.New(), uuid.New()
	stub := &serviceKeyStub{}
	server, _ := serviceKeyServer(t, actor, []string{"admin"}, stub)
	path := "/api/v1/admin/service-keys/" + id.String()

	for range 2 {
		if response := adminRequest(t, server, http.MethodDelete, path, "", actor); response.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
	if stub.revokedID != id || stub.revokeInput.ActorUserID != actor || !stub.revokeInput.IP.IsValid() {
		t.Fatalf("revoke id=%v input=%+v", stub.revokedID, stub.revokeInput)
	}
	stub.err = servicekey.ErrNotFound
	if response := adminRequest(t, server, http.MethodDelete, path, "", actor); response.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", response.Code)
	}
}

func TestRF018_SoloUnAdminGestionaClaves(t *testing.T) {
	actor := uuid.New()
	stub := &serviceKeyStub{}
	server, _ := serviceKeyServer(t, actor, []string{"user"}, stub)
	id := uuid.New().String()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/admin/service-keys", `{"name":"x","scopes":["audit-log:read"],"expiresInDays":3}`},
		{http.MethodGet, "/api/v1/admin/service-keys", ""},
		{http.MethodDelete, "/api/v1/admin/service-keys/" + id, ""},
	} {
		if response := adminRequest(t, server, tc.method, tc.path, tc.body, actor); response.Code != http.StatusForbidden {
			t.Fatalf("%s %s status=%d, want 403", tc.method, tc.path, response.Code)
		}
		anonymous := httptest.NewRecorder()
		server.Routes().ServeHTTP(anonymous, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if anonymous.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s anonymous status=%d, want 401", tc.method, tc.path, anonymous.Code)
		}
	}
	if stub.createCalled != 0 || stub.listCalls != 0 {
		t.Fatal("a non-admin must not reach the service")
	}
}

func TestRF018_ClaveConAlcanceListaElAuditLog(t *testing.T) {
	stub := &serviceKeyStub{}
	server, logs := serviceKeyServer(t, uuid.New(), []string{"admin"}, stub)

	response := serviceKeyBearer(server, http.MethodGet, "/api/v1/admin/audit-log?limit=5", "", sampleBearer)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "login_failed") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if stub.authRaw != sampleBearer || stub.authScope != servicekey.ScopeAuditLogRead || !stub.authMeta.IP.IsValid() || stub.authMeta.UserAgent != "siem-tests" {
		t.Fatalf("authenticate called with raw=%q scope=%q meta=%+v", stub.authRaw, stub.authScope, stub.authMeta)
	}
	if logs.input.Limit != 6 {
		t.Fatalf("limit=%d, want the page size to reach the audit service", logs.input.Limit)
	}
}

func TestRF018_ClaveInvalidaRecibeSiempreElMismo401(t *testing.T) {
	var first string
	for name, authErr := range map[string]error{"unauthenticated": servicekey.ErrUnauthenticated, "wrapped": errors.Join(servicekey.ErrUnauthenticated)} {
		stub := &serviceKeyStub{authErr: authErr}
		server, _ := serviceKeyServer(t, uuid.New(), []string{"admin"}, stub)
		response := serviceKeyBearer(server, http.MethodGet, "/api/v1/admin/audit-log", "", sampleBearer)
		if response.Code != http.StatusUnauthorized || !strings.Contains(response.Header().Get("Content-Type"), "problem+json") {
			t.Fatalf("%s: status=%d body=%s", name, response.Code, response.Body.String())
		}
		if first == "" {
			first = response.Body.String()
		} else if response.Body.String() != first {
			t.Fatalf("response differs between failures: %s vs %s", first, response.Body.String())
		}
	}
}

func TestRF018_ClaveSinElAlcanceExigidoRecibe403(t *testing.T) {
	stub := &serviceKeyStub{authErr: servicekey.ErrInsufficientScope}
	server, _ := serviceKeyServer(t, uuid.New(), []string{"admin"}, stub)
	if response := serviceKeyBearer(server, http.MethodGet, "/api/v1/admin/audit-log", "", sampleBearer); response.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", response.Code)
	}
}

func TestRF018_ClaveNoAbreOtrasOperacionesDeAdministracion(t *testing.T) {
	stub := &serviceKeyStub{}
	server, _ := serviceKeyServer(t, uuid.New(), []string{"admin"}, stub)
	id := uuid.New().String()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/users", ""},
		{http.MethodGet, "/api/v1/admin/users/" + id, ""},
		{http.MethodGet, "/api/v1/admin/applications", ""},
		{http.MethodGet, "/api/v1/admin/service-keys", ""},
		{http.MethodPost, "/api/v1/admin/service-keys", `{"name":"x","scopes":["audit-log:read"],"expiresInDays":3}`},
		{http.MethodDelete, "/api/v1/admin/service-keys/" + id, ""},
		{http.MethodGet, "/api/v1/me", ""},
	} {
		response := serviceKeyBearer(server, tc.method, tc.path, tc.body, sampleBearer)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d, want 401", tc.method, tc.path, response.Code)
		}
	}
	if stub.authCalls != 0 || stub.createCalled != 0 || stub.listCalls != 0 {
		t.Fatal("a service key must not be honored outside operations that declare its scope")
	}
}

func TestRF018_TokenHumanoSigueFuncionandoEnElAuditLog(t *testing.T) {
	actor := uuid.New()
	stub := &serviceKeyStub{}
	server, _ := serviceKeyServer(t, actor, []string{"admin"}, stub)
	if response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log", "", actor); response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	if stub.authCalls != 0 {
		t.Fatal("a human access token must not go through service key authentication")
	}
}

func TestRF018_SinServicioConfiguradoLaClaveNoAbreNada(t *testing.T) {
	server := auditLogServer(t, uuid.New(), []string{"admin"}, &auditLogServiceStub{})
	if response := serviceKeyBearer(server, http.MethodGet, "/api/v1/admin/audit-log", "", sampleBearer); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", response.Code)
	}
}
