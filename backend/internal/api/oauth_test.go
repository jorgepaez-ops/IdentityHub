package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

func TestRF020_SinSesionRedirigeAlLoginConContinueRelativo(t *testing.T) {
	server, client := oauthAPIServer(t, nil)
	request := httptest.NewRequest(http.MethodGet, oauthAuthorizePath(client), nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	location, _ := url.Parse(response.Header().Get("Location"))
	if location.Host != "identityhub.localhost:8080" || !strings.HasPrefix(location.Query().Get("continue"), "/oauth/authorize?") {
		t.Fatalf("location=%s", location)
	}
}

func TestRF020_SesionHubAusenteRedirigePeroFalloDeStoreDa500YSeRegistra(t *testing.T) {
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}
	for _, testCase := range []struct {
		name       string
		sessionErr error
		wantStatus int
	}{
		{"missing", oauth.ErrHubSessionInvalid, http.StatusFound},
		{"storage failure", errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var logs bytes.Buffer
			server := NewServer(slog.New(slog.NewTextHandler(&logs, nil)), "test", nil)
			server.SetOAuthService(oauth.New(&oauthAPIRepository{}, []oauth.Client{client}, nil, time.Now), hubSessionStub{err: testCase.sessionErr}, "http://identityhub.localhost:8080/login", time.Hour)
			request := httptest.NewRequest(http.MethodGet, oauthAuthorizePath(client), nil)
			request.AddCookie(&http.Cookie{Name: hubSessionCookieName, Value: "c2VjcmV0LWh1Yi10b2tlbg"})
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if testCase.wantStatus == http.StatusInternalServerError {
				if !strings.Contains(logs.String(), "oauth request failed") || strings.Contains(logs.String(), "c2VjcmV0") {
					t.Fatalf("logs=%q", logs.String())
				}
			}
		})
	}
}

func TestRF020_ClienteInvalidoNoRedirigeALaURIRecibida(t *testing.T) {
	server, client := oauthAPIServer(t, nil)
	path := oauthAuthorizePath(client) + "&redirect_uri=" + url.QueryEscape("https://evil.example/callback")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusBadRequest || strings.Contains(response.Header().Get("Location"), "evil.example") {
		t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
}

func TestRF020_ParametrosOAuthInvalidosRedirigenSoloClienteValidado(t *testing.T) {
	server, client := oauthAPIServer(t, nil)
	for _, path := range []string{
		strings.Replace(oauthAuthorizePath(client), "response_type=code", "response_type=token", 1),
		strings.Replace(oauthAuthorizePath(client), "code_challenge="+strings.Repeat("a", 43), "code_challenge=short", 1),
	} {
		response := httptest.NewRecorder()
		server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		location, _ := url.Parse(response.Header().Get("Location"))
		if response.Code != http.StatusFound || location.Query().Get("error") != "invalid_request" || location.Query().Get("state") != "state" {
			t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
		}
	}
}

func TestRF020_ParametrosOAuthFaltantesRedirigenSoloClienteValidado(t *testing.T) {
	server, client := oauthAPIServer(t, nil)
	for _, parameter := range []string{"response_type=code", "state=state", "code_challenge=" + strings.Repeat("a", 43), "code_challenge_method=S256"} {
		path := strings.Replace(oauthAuthorizePath(client), "&"+parameter, "", 1)
		response := httptest.NewRecorder()
		server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		location, _ := url.Parse(response.Header().Get("Location"))
		if response.Code != http.StatusFound || location.Query().Get("error") != "invalid_request" {
			t.Fatalf("missing %s: status=%d location=%q", parameter, response.Code, response.Header().Get("Location"))
		}
	}
}

func TestRF020_ParametrosOAuthDuplicadosNoRedirigen(t *testing.T) {
	server, client := oauthAPIServer(t, nil)
	for _, duplicate := range []string{"client_id=contabilidad", "redirect_uri=" + url.QueryEscape(client.RedirectURI), "state=other"} {
		response := httptest.NewRecorder()
		server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, oauthAuthorizePath(client)+"&"+duplicate, nil))
		if response.Code != http.StatusBadRequest || response.Header().Get("Location") != "" {
			t.Fatalf("duplicate %s: status=%d location=%q", duplicate, response.Code, response.Header().Get("Location"))
		}
	}
}

func TestRF020_CORSSeLimitaATokenYJWKS(t *testing.T) {
	server, client := oauthAPIServer(t, nil)
	for _, testCase := range []struct {
		path string
		want bool
	}{{"/oauth/token", true}, {"/.well-known/jwks.json", true}, {"/healthz", false}} {
		request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
		request.Header.Set("Origin", client.Origin)
		response := httptest.NewRecorder()
		server.Routes().ServeHTTP(response, request)
		if (response.Header().Get("Access-Control-Allow-Origin") == client.Origin) != testCase.want {
			t.Fatalf("path=%s cors=%q", testCase.path, response.Header().Get("Access-Control-Allow-Origin"))
		}
		if response.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("credentials enabled on %s", testCase.path)
		}
	}
}

func TestRF020_CORSSiempreVaryOriginYNuncaCredenciales(t *testing.T) {
	server, client := oauthAPIServer(t, nil)
	for _, path := range []string{"/oauth/token", "/.well-known/jwks.json"} {
		for _, origin := range []string{client.Origin, "https://evil.example", ""} {
			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodOptions} {
				assertCORSHeaders(t, server, method, path, origin, client.Origin)
			}
		}
	}
}

// assertCORSHeaders sends one request and checks Vary: Origin is always present, credentials are
// never enabled and only the registered origin is ever allowed.
func assertCORSHeaders(t *testing.T, server *Server, method, path, origin, allowedOrigin string) {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if !strings.Contains(response.Header().Get("Vary"), "Origin") {
		t.Fatalf("%s %s origin=%q Vary=%q", method, path, origin, response.Header().Values("Vary"))
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("%s %s origin=%q enabled credentials", method, path, origin)
	}
	if origin != allowedOrigin && response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("%s %s origin=%q allowed", method, path, origin)
	}
}

func TestRF020_TokenEndpointResponseNoStore(t *testing.T) {
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	verifier := strings.Repeat("v", 43)
	form := func(code string) string {
		return url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {client.ID}, "redirect_uri": {client.RedirectURI}, "code_verifier": {verifier}}.Encode()
	}
	for _, testCase := range []struct {
		name   string
		repo   *exchangeRepository
		body   string
		status int
	}{
		{"success", &exchangeRepository{}, form("AQID"), http.StatusOK},
		{"invalid grant", &exchangeRepository{err: oauth.ErrAuthorizationCodeInvalid}, form("AQID"), http.StatusBadRequest},
		{"invalid request", &exchangeRepository{}, "grant_type=password", http.StatusBadRequest},
		{"repository failure", &exchangeRepository{err: errors.New("database unavailable")}, form("AQID"), http.StatusInternalServerError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := NewServer(nil, "test", nil)
			server.SetTokenService(signer)
			server.SetOAuthService(oauth.New(testCase.repo, []oauth.Client{client}, nil, time.Now), invalidHubSession{}, "http://identityhub.localhost:8080/login", time.Hour)
			request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, request)
			if response.Code != testCase.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if got := response.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control=%q", got)
			}
			if testCase.status != http.StatusOK && testCase.repo.audits != 0 {
				t.Fatalf("unexpected audits=%d", testCase.repo.audits)
			}
		})
	}
}

type exchangeRepository struct {
	err    error
	audits int
	roles  map[string][]string
}

func (r *exchangeRepository) WithinAuthorizationCodeTransaction(_ context.Context, fn func(oauth.ExchangeWriter) error) error {
	return fn(r)
}

func (r *exchangeRepository) CreateAuthorizationCode(context.Context, oauth.CreateCode) error {
	return nil
}
func (r *exchangeRepository) ExchangeAuthorizationCode(context.Context, []byte, string, string, string, time.Time) (oauth.StoredCode, error) {
	if r.err != nil {
		return oauth.StoredCode{}, r.err
	}
	return oauth.StoredCode{UserID: uuid.New()}, nil
}
func (r *exchangeRepository) ListApplicationRolesForUser(_ context.Context, _ uuid.UUID, clientID string) ([]string, error) {
	return r.roles[clientID], nil
}
func (r *exchangeRepository) GetAuthorizationAccess(context.Context, uuid.UUID, string) (oauth.AuthorizationAccess, error) {
	return oauth.AuthorizationAccess{}, nil
}
func (r *exchangeRepository) ListPermissionKeysForRolesAndApplication(context.Context, []string, string) ([]string, error) {
	return nil, nil
}
func (r *exchangeRepository) InsertAuditEvent(context.Context, oauth.AuditEvent) error {
	r.audits++
	return nil
}

func TestRF014_VerificarMFAEmiteCookieDeSesionHub(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetMFAService(&mfaStub{result: mfaResultWithHubSession()}, time.Hour)
	server.SetOAuthService(nil, nil, "http://identityhub.localhost:8080/login", time.Hour)
	response := httptest.NewRecorder()
	server.VerifyMfa(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{"mfaToken":"challenge","code":"123456"}`)))
	var hub *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == hubSessionCookieName {
			hub = cookie
		}
	}
	if hub == nil || !hub.HttpOnly || !hub.Secure || hub.SameSite != http.SameSiteLaxMode || hub.Path != "/oauth" {
		t.Fatalf("hub cookie=%+v", hub)
	}
}

func oauthAPIServer(t *testing.T, session oauth.HubSessionReader) (*Server, oauth.Client) {
	t.Helper()
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}
	repo := &oauthAPIRepository{}
	server := NewServer(nil, "test", nil)
	server.SetOAuthService(oauth.New(repo, []oauth.Client{client}, strings.NewReader(strings.Repeat("x", 64)), time.Now), sessionOrInvalid(session), "http://identityhub.localhost:8080/login", time.Hour)
	return server, client
}
func sessionOrInvalid(session oauth.HubSessionReader) oauth.HubSessionReader {
	if session != nil {
		return session
	}
	return invalidHubSession{}
}
func oauthAuthorizePath(client oauth.Client) string {
	return "/oauth/authorize?client_id=" + client.ID + "&redirect_uri=" + url.QueryEscape(client.RedirectURI) + "&response_type=code&state=state&code_challenge=" + strings.Repeat("a", 43) + "&code_challenge_method=S256"
}
func mfaResultWithHubSession() mfa.Result {
	return mfa.Result{AccessToken: "access", RefreshToken: "refresh", HubSessionToken: "hub", TokenType: "Bearer", ExpiresIn: 900}
}

type invalidHubSession struct{}

func (invalidHubSession) GetHubSessionUser(context.Context, []byte) (uuid.UUID, error) {
	return uuid.Nil, oauth.ErrHubSessionInvalid
}

type hubSessionStub struct{ err error }

func (s hubSessionStub) GetHubSessionUser(context.Context, []byte) (uuid.UUID, error) {
	return uuid.Nil, s.err
}

type oauthAPIRepository struct {
	access *oauth.AuthorizationAccess
	audits []oauth.AuditEvent
}

func (r *oauthAPIRepository) WithinAuthorizationCodeTransaction(_ context.Context, fn func(oauth.ExchangeWriter) error) error {
	return fn(r)
}

func (*oauthAPIRepository) CreateAuthorizationCode(context.Context, oauth.CreateCode) error {
	return nil
}
func (*oauthAPIRepository) ExchangeAuthorizationCode(context.Context, []byte, string, string, string, time.Time) (oauth.StoredCode, error) {
	return oauth.StoredCode{}, oauth.ErrAuthorizationCodeInvalid
}
func (r *oauthAPIRepository) ListApplicationRolesForUser(context.Context, uuid.UUID, string) ([]string, error) {
	return nil, nil
}

// GetAuthorizationAccess defaults to a user of the application's own company holding a role.
func (r *oauthAPIRepository) GetAuthorizationAccess(context.Context, uuid.UUID, string) (oauth.AuthorizationAccess, error) {
	if r.access != nil {
		return *r.access, nil
	}
	return oauth.AuthorizationAccess{HasApplicationRole: true}, nil
}
func (*oauthAPIRepository) ListPermissionKeysForRolesAndApplication(context.Context, []string, string) ([]string, error) {
	return nil, nil
}
func (r *oauthAPIRepository) InsertAuditEvent(_ context.Context, event oauth.AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

func TestRF020_AutorizacionSinOAuthConfiguradoNoRedirigeNiFalla(t *testing.T) {
	for name, configure := range map[string]func(*Server){
		"not wired":   func(*Server) {},
		"nil service": func(s *Server) { s.SetOAuthService(nil, nil, "", time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			server := NewServer(nil, "test", nil)
			configure(server)
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id=&redirect_uri=", nil))
			if response.Code == http.StatusFound || response.Code >= http.StatusInternalServerError && response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d location=%q, want a 4xx or 503 without redirect", response.Code, response.Header().Get("Location"))
			}
		})
	}
}

func inventarioAPIClient() oauth.Client {
	return oauth.Client{ID: "inventario", RedirectURI: "http://inventario.localhost:8080/oauth/callback", Origin: "http://inventario.localhost:8080"}
}

type hubSessionUser struct{ id uuid.UUID }

func (s hubSessionUser) GetHubSessionUser(context.Context, []byte) (uuid.UUID, error) {
	return s.id, nil
}

func TestRF022_AuthorizeRechazadoRedirigeConAccessDeniedYAudita(t *testing.T) {
	user := uuid.New()
	for _, testCase := range []struct {
		name       string
		access     oauth.AuthorizationAccess
		wantReason string
	}{
		{"otra empresa", oauth.AuthorizationAccess{UserOrganizationID: uuid.New(), ApplicationOrganizationID: uuid.New(), HasApplicationRole: true}, oauth.RejectionOrganizationMismatch},
		{"sin rol de la aplicación", oauth.AuthorizationAccess{}, oauth.RejectionNoApplicationRole},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &oauthAPIRepository{access: &testCase.access}
			client := inventarioAPIClient()
			server := NewServer(nil, "test", nil)
			server.SetOAuthService(oauth.New(repo, []oauth.Client{client}, strings.NewReader(strings.Repeat("x", 64)), time.Now), hubSessionUser{id: user}, "http://identityhub.localhost:8080/login", time.Hour)
			request := httptest.NewRequest(http.MethodGet, oauthAuthorizePath(client), nil)
			request.AddCookie(&http.Cookie{Name: hubSessionCookieName, Value: "c2VjcmV0LWh1Yi10b2tlbg"})
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, request)
			location, _ := url.Parse(response.Header().Get("Location"))
			if response.Code != http.StatusFound || location.Scheme+"://"+location.Host+location.Path != client.RedirectURI {
				t.Fatalf("status=%d location=%q, want a redirect to the registered redirect URI", response.Code, response.Header().Get("Location"))
			}
			if location.Query().Get("error") != "access_denied" || location.Query().Get("state") != "state" || location.Query().Get("code") != "" {
				t.Fatalf("query=%v, want error=access_denied, original state and no code", location.Query())
			}
			if len(repo.audits) != 1 || repo.audits[0].Action != "oauth_authorization_rejected" || repo.audits[0].ActorUserID != user || repo.audits[0].Metadata["reason"] != testCase.wantReason || repo.audits[0].Metadata["client_id"] != "inventario" {
				t.Fatalf("audits=%+v", repo.audits)
			}
		})
	}
}

func TestRF022_AuthorizeConClienteOUriInvalidosNoRedirigeNiAudita(t *testing.T) {
	repo := &oauthAPIRepository{access: &oauth.AuthorizationAccess{}}
	client := inventarioAPIClient()
	server := NewServer(nil, "test", nil)
	server.SetOAuthService(oauth.New(repo, []oauth.Client{client}, nil, time.Now), hubSessionUser{id: uuid.New()}, "http://identityhub.localhost:8080/login", time.Hour)
	for name, path := range map[string]string{
		"redirect ajeno":   oauthAuthorizePath(client) + "&redirect_uri=" + url.QueryEscape("https://evil.example/cb"),
		"cliente inexiste": strings.Replace(oauthAuthorizePath(client), "client_id=inventario", "client_id=nope", 1),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.AddCookie(&http.Cookie{Name: hubSessionCookieName, Value: "c2VjcmV0LWh1Yi10b2tlbg"})
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, request)
			if response.Code == http.StatusFound || response.Header().Get("Location") != "" || len(repo.audits) != 0 {
				t.Fatalf("status=%d location=%q audits=%d", response.Code, response.Header().Get("Location"), len(repo.audits))
			}
		})
	}
}

func TestRF020_CORSPermiteElOrigenDeCadaClienteRegistradoYNingunOtro(t *testing.T) {
	contabilidad, inventario := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}, inventarioAPIClient()
	server := NewServer(nil, "test", nil)
	server.SetOAuthService(oauth.New(&oauthAPIRepository{}, []oauth.Client{contabilidad, inventario}, nil, time.Now), invalidHubSession{}, "http://identityhub.localhost:8080/login", time.Hour)
	for _, path := range []string{"/oauth/token", "/.well-known/jwks.json"} {
		for origin, allowed := range map[string]bool{contabilidad.Origin: true, inventario.Origin: true, "https://evil.example": false, "*": false, "null": false} {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Origin", origin)
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, request)
			got := response.Header().Get("Access-Control-Allow-Origin")
			if allowed && got != origin || !allowed && got != "" {
				t.Fatalf("%s origin=%q allow-origin=%q, allowed=%v", path, origin, got, allowed)
			}
		}
	}
	preflight := httptest.NewRequest(http.MethodOptions, "/oauth/token", nil)
	preflight.Header.Set("Origin", inventario.Origin)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, preflight)
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != inventario.Origin {
		t.Fatalf("preflight status=%d allow-origin=%q", response.Code, response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRF022_TokenDeInventarioTieneAudInventarioYSoloSusRoles(t *testing.T) {
	client := inventarioAPIClient()
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	repo := &exchangeRepository{roles: map[string][]string{"inventario": {"inventario.bodeguero"}, "contabilidad": {"contabilidad.senior"}}}
	server := NewServer(nil, "test", nil)
	server.SetTokenService(signer)
	other := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}
	server.SetOAuthService(oauth.New(repo, []oauth.Client{other, client}, nil, time.Now), invalidHubSession{}, "http://identityhub.localhost:8080/login", time.Hour)
	body := url.Values{"grant_type": {"authorization_code"}, "code": {"AQID"}, "client_id": {client.ID}, "redirect_uri": {client.RedirectURI}, "code_verifier": {strings.Repeat("v", 43)}}.Encode()
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &tokenResponse); err != nil || tokenResponse.AccessToken == "" {
		t.Fatalf("body=%s err=%v", response.Body.String(), err)
	}
	segments := strings.Split(tokenResponse.AccessToken, ".")
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Aud   []string `json:"aud"`
		Roles []string `json:"roles"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Aud) != 1 || claims.Aud[0] != "inventario" || len(claims.Roles) != 1 || claims.Roles[0] != "inventario.bodeguero" {
		t.Fatalf("claims=%+v, want aud=[inventario] and only inventario roles", claims)
	}
}
