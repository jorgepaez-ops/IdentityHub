package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/auth/logout"
)

type logoutStub struct {
	err   error
	input logout.Input
}

func (s *logoutStub) Logout(_ context.Context, input logout.Input) error {
	s.input = input
	return s.err
}

func logoutServer(service logout.Revoker) *Server {
	server := NewServer(nil, "test", nil)
	server.SetLogoutService(service)
	return server
}

func TestRF007_LogoutRevocaElRefreshToken(t *testing.T) {
	service := &logoutStub{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
	recorder := httptest.NewRecorder()

	logoutServer(service).Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body=%q, want empty", recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != refreshCookieName || cookies[0].Value != "" || cookies[0].MaxAge >= 0 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/api/v1/auth" {
		t.Fatalf("cookies=%+v", cookies)
	}
	if service.input.RefreshToken != "refresh-token" || service.input.IP == nil {
		t.Fatalf("input=%+v", service.input)
	}
}

func TestRF007_LogoutDeTokenDesconocidoDevuelve401(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "unknown"})
	recorder := httptest.NewRecorder()

	logoutServer(&logoutStub{err: logout.ErrInvalidRefreshToken}).Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "" || cookies[0].MaxAge >= 0 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/api/v1/auth" {
		t.Fatalf("cookies=%+v", cookies)
	}
}

func TestRF007_LogoutSinCookieDevuelve401(t *testing.T) {
	recorder := httptest.NewRecorder()

	logoutServer(&logoutStub{}).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "" || cookies[0].MaxAge >= 0 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/api/v1/auth" {
		t.Fatalf("cookies=%+v", cookies)
	}
}

func TestRF007_LogoutHandlerRechazaErroresNoAutorizados(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
	recorder := httptest.NewRecorder()

	logoutServer(&logoutStub{err: errors.New("database unavailable")}).Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recorder.Code)
	}
}
