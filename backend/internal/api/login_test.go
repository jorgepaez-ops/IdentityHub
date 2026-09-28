package api

import (
	"context"
	"fmt"
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
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
)

type loginStub struct {
	result login.Result
	err    error
}

type mfaStub struct {
	result               mfa.Result
	verifyErr, resendErr error
	verifyInput          mfa.VerifyInput
}

type mfaIssuerStub struct{}

func (mfaIssuerStub) Issue(context.Context, mfa.User) (mfa.Challenge, error) {
	return mfa.Challenge{Token: "challenge", ExpiresIn: 300}, nil
}

func (s *mfaStub) Verify(_ context.Context, input mfa.VerifyInput) (mfa.Result, error) {
	s.verifyInput = input
	return s.result, s.verifyErr
}
func (s *mfaStub) Resend(context.Context, string) error { return s.resendErr }

func (s loginStub) Login(context.Context, login.Input) (login.Result, error) { return s.result, s.err }

func TestRF013_LoginCorrectoDevuelveDesafioMFAyNoCookie(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{result: login.Result{MfaToken: "short-lived-token", ExpiresIn: 300}})
	response := httptest.NewRecorder()
	server.Login(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`)))
	if response.Code != http.StatusAccepted || len(response.Result().Cookies()) != 0 || !strings.Contains(response.Body.String(), "mfaToken") {
		t.Fatalf("status=%d cookies=%v body=%s", response.Code, response.Result().Cookies(), response.Body.String())
	}
}

func TestRF014_VerificarMFAEntregaTokensYCookieRefresh(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetMFAService(&mfaStub{result: mfa.Result{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 900}}, time.Hour)
	response := httptest.NewRecorder()
	server.VerifyMfa(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{"mfaToken":"challenge","code":"123456"}`)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "access") || len(response.Result().Cookies()) != 1 {
		t.Fatalf("status=%d body=%s cookies=%v", response.Code, response.Body.String(), response.Result().Cookies())
	}
	if strings.Contains(response.Body.String(), "refresh") {
		t.Fatalf("refresh token leaked in the JSON body: %s", response.Body.String())
	}
	cookie := response.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/v1/auth" || cookie.MaxAge != int(time.Hour.Seconds()) {
		t.Fatalf("cookie = %+v", cookie)
	}
}

func TestRF014_VerificarMFAConservaIPConfiableYAgenteEnLaSesion(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetTrustedProxies([]netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")})
	mfaService := &mfaStub{result: mfa.Result{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 900}}
	server.SetMFAService(mfaService, time.Hour)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{"mfaToken":"challenge","code":"123456"}`))
	request.RemoteAddr = "127.0.0.1:4242"
	request.Header.Set("X-Forwarded-For", "203.0.113.8")
	request.Header.Set("User-Agent", "Identity Hub test client")
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	wantIP := netip.MustParseAddr("203.0.113.8")
	if response.Code != http.StatusOK || mfaService.verifyInput.IP == nil || *mfaService.verifyInput.IP != wantIP {
		t.Fatalf("status=%d verify input=%+v, want trusted IP %v", response.Code, mfaService.verifyInput, wantIP)
	}
	if mfaService.verifyInput.UserAgent == nil || *mfaService.verifyInput.UserAgent != "Identity Hub test client" {
		t.Fatalf("verify user-agent=%v", mfaService.verifyInput.UserAgent)
	}
}

func TestRF014_ReenvioTempranoDevuelve429(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetMFAService(&mfaStub{resendErr: mfa.ErrResendTooSoon}, time.Hour)
	response := httptest.NewRecorder()
	server.ResendMfaCode(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/resend", strings.NewReader(`{"mfaToken":"challenge"}`)))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF014_CamposMFAObligatoriosVaciosDevuelven400(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetMFAService(&mfaStub{}, time.Hour)
	for _, body := range []string{`{"mfaToken":"","code":"123456"}`, `{"mfaToken":"challenge","code":""}`, `{"mfaToken":""}`} {
		response := httptest.NewRecorder()
		server.VerifyMfa(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
}

func TestRF003_PasswordIncorrectoDevuelve401Generico(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{err: login.ErrInvalidCredentials})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong"}`))
	server.Login(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "ada@example.com") {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
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
func (*realLoginRepositoryStub) UpdatePasswordHash(context.Context, uuid.UUID, string) error {
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
			server.SetLoginService(login.New(testCase.repo).WithMFA(mfaIssuerStub{}))
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
		server.SetLoginService(loginStub{err: err})
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
	server.SetLoginService(authenticator)
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
		server.SetLoginService(authenticator)
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
		server.SetLoginService(authenticator)
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

func TestRF014_LaEmisionLimitadaDeLoginDevuelve429ProblemJSON(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{err: fmt.Errorf("issue mfa challenge: %w", mfa.ErrIssuanceLimited)})
	response := httptest.NewRecorder()
	server.Login(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`)))
	if response.Code != http.StatusTooManyRequests || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/problem+json") {
		t.Fatalf("status=%d content-type=%q body=%s, want 429 problem+json", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
}

func TestRF014_FalloDeEntregaDelCodigoDevuelve503ProblemJSON(t *testing.T) {
	deliveryErr := fmt.Errorf("issue mfa challenge: %w", mfa.ErrDeliveryUnavailable)
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{err: deliveryErr})
	server.SetMFAService(&mfaStub{resendErr: deliveryErr}, time.Hour)
	for name, call := range map[string]func(http.ResponseWriter){
		"login": func(w http.ResponseWriter) {
			server.Login(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`)))
		},
		"resend": func(w http.ResponseWriter) {
			server.ResendMfaCode(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/resend", strings.NewReader(`{"mfaToken":"challenge"}`)))
		},
	} {
		response := httptest.NewRecorder()
		call(response)
		if response.Code != http.StatusServiceUnavailable || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatalf("%s: status=%d content-type=%q body=%s, want 503 problem+json", name, response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
	}
}
