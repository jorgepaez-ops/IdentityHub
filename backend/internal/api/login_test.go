package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type loginStub struct {
	result login.Result
	err    error
}

func (s loginStub) Login(context.Context, login.Input) (login.Result, error) { return s.result, s.err }

func TestRF003_LoginCorrectoDevuelveParDeTokens(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{result: login.Result{AccessToken: "access-token", TokenType: "Bearer", ExpiresIn: 900, RefreshToken: "opaque-refresh"}}, 720*time.Hour)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`))

	server.Login(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["accessToken"] != "access-token" || body["refreshToken"] != nil {
		t.Fatalf("body = %#v", body)
	}
	cookie := response.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/v1/auth" {
		t.Fatalf("cookie = %+v", cookie)
	}
}

func TestRF003_PasswordIncorrectoDevuelve401Generico(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{err: login.ErrInvalidCredentials}, time.Hour)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong"}`))
	server.Login(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "ada@example.com") {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestRF003_CuentaConMFARechazadaConErrorClaro(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{err: errors.Join(login.ErrMFAUnavailable, errors.New("internal context"))}, time.Hour)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`))
	server.Login(response, request)
	if response.Code != http.StatusNotImplemented || len(response.Result().Cookies()) != 0 || !strings.Contains(response.Body.String(), "MFA is not supported") {
		t.Fatalf("response = %d cookies=%v %s", response.Code, response.Result().Cookies(), response.Body.String())
	}
}

type realLoginRepositoryStub struct {
	user      login.User
	lookupErr error
	audits    []login.AuditEvent
}

func (s *realLoginRepositoryStub) WithinLoginTransaction(_ context.Context, fn func(login.Writer) error) error {
	return fn(s)
}
func (s *realLoginRepositoryStub) GetLoginUserByEmail(context.Context, string) (login.User, error) {
	return s.user, s.lookupErr
}
func (*realLoginRepositoryStub) CountLoginFailuresByAccount(context.Context, uuid.UUID, time.Time) (int64, error) {
	return 0, nil
}
func (*realLoginRepositoryStub) CountLoginFailuresByIP(context.Context, netip.Addr, time.Time) (int64, error) {
	return 0, nil
}
func (*realLoginRepositoryStub) LockLoginUser(context.Context, uuid.UUID, time.Time) error {
	return nil
}
func (*realLoginRepositoryStub) UnlockLoginUser(context.Context, uuid.UUID) error { return nil }
func (*realLoginRepositoryStub) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	return []string{"user"}, nil
}
func (*realLoginRepositoryStub) UpdateLoginSuccess(context.Context, uuid.UUID, string) error {
	return nil
}
func (*realLoginRepositoryStub) CreateRefreshToken(context.Context, login.RefreshToken) error {
	return nil
}
func (s *realLoginRepositoryStub) InsertAuditEvent(_ context.Context, event login.AuditEvent) error {
	s.audits = append(s.audits, event)
	return nil
}

func TestRF003_AM004EmailInexistenteYPasswordIncorrectoDevuelvenElMismoMensaje(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		repo *realLoginRepositoryStub
	}{
		{name: "unknown email", repo: &realLoginRepositoryStub{lookupErr: pgx.ErrNoRows}},
		{name: "wrong password", repo: &realLoginRepositoryStub{user: login.User{ID: uuid.New(), PasswordHash: hash, Status: login.StatusActive}}},
	}
	var responseBodies []string
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := NewServer(nil, "test", nil)
			server.SetLoginService(login.New(testCase.repo, signer, time.Hour), time.Hour)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong password"}`))
			server.Login(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
			}
			responseBodies = append(responseBodies, response.Body.String())
		})
	}
	if responseBodies[0] != responseBodies[1] {
		t.Fatalf("responses disclose account existence: unknown=%s wrong-password=%s", responseBodies[0], responseBodies[1])
	}
}

func TestRF017_BloqueoDevuelve423SinRevelarSuOrigen(t *testing.T) {
	responses := make([]string, 0, 2)
	for _, err := range []error{login.ErrAccountLocked, login.ErrIPRateLimited} {
		server := NewServer(nil, "test", nil)
		server.SetLoginService(loginStub{err: err}, time.Hour)
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`))
		server.Login(response, request)
		if response.Code != http.StatusLocked {
			t.Fatalf("status = %d, want 423: %s", response.Code, response.Body.String())
		}
		responses = append(responses, response.Body.String())
	}
	if responses[0] != responses[1] {
		t.Fatalf("lock response disclosed its origin: account=%s ip=%s", responses[0], responses[1])
	}
}

type rf017RouteAuthenticator struct {
	byIP      bool
	threshold int
	counts    map[string]int
	seenIPs   []netip.Addr
}

func (a *rf017RouteAuthenticator) Login(_ context.Context, input login.Input) (login.Result, error) {
	key := input.Email
	if input.IP != nil {
		a.seenIPs = append(a.seenIPs, *input.IP)
		if a.byIP {
			key = input.IP.String()
		}
	}
	a.counts[key]++
	if a.counts[key] > a.threshold {
		if a.byIP {
			return login.Result{}, login.ErrIPRateLimited
		}
		return login.Result{}, login.ErrAccountLocked
	}
	return login.Result{}, login.ErrInvalidCredentials
}

func rf017RouteRequest(header string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong password"}`))
	req.RemoteAddr = "203.0.113.10:4567"
	req.Header.Set("X-Forwarded-For", header)
	return req
}

func TestRF017_BloqueoNoSeEvitaFalsificandoXForwardedFor(t *testing.T) {
	authenticator := &rf017RouteAuthenticator{threshold: 5, counts: make(map[string]int)}
	server := NewServer(nil, "test", nil)
	server.SetLoginService(authenticator, time.Hour)
	handler := server.Routes()
	for attempt := 1; attempt <= 6; attempt++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, rf017RouteRequest("198.51.100."+strconv.Itoa(attempt)))
		want := http.StatusUnauthorized
		if attempt == 6 {
			want = http.StatusLocked
		}
		if response.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, want)
		}
	}
	for _, ip := range authenticator.seenIPs {
		if ip != netip.MustParseAddr("203.0.113.10") {
			t.Fatalf("untrusted X-Forwarded-For changed client IP: %v", authenticator.seenIPs)
		}
	}
}

func TestRF017_RotarXForwardedForNoEvadeElLimitePorIP(t *testing.T) {
	t.Run("untrusted peer", func(t *testing.T) {
		authenticator := &rf017RouteAuthenticator{byIP: true, threshold: 2, counts: make(map[string]int)}
		server := NewServer(nil, "test", nil)
		server.SetLoginService(authenticator, time.Hour)
		handler := server.Routes()
		for attempt := 1; attempt <= 3; attempt++ {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, rf017RouteRequest("198.51.100."+strconv.Itoa(attempt)))
			want := http.StatusUnauthorized
			if attempt == 3 {
				want = http.StatusLocked
			}
			if response.Code != want {
				t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, want)
			}
		}
	})
	t.Run("trusted proxy separates clients", func(t *testing.T) {
		authenticator := &rf017RouteAuthenticator{byIP: true, threshold: 2, counts: make(map[string]int)}
		server := NewServer(nil, "test", nil)
		server.SetLoginService(authenticator, time.Hour)
		server.SetTrustedProxies([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")})
		handler := server.Routes()
		for _, header := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.1"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, rf017RouteRequest(header))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("header %s status = %d, want 401", header, response.Code)
			}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, rf017RouteRequest("198.51.100.1"))
		if response.Code != http.StatusLocked {
			t.Fatalf("third client-1 failure status = %d, want 423", response.Code)
		}
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, rf017RouteRequest("198.51.100.2"))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("independent client-2 status = %d, want 401", response.Code)
		}
	})
}
