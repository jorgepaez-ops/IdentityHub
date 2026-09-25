package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
)

type refreshStub struct {
	result refresh.Result
	err    error
	input  refresh.Input
}

func (s *refreshStub) Refresh(_ context.Context, input refresh.Input) (refresh.Result, error) {
	s.input = input
	return s.result, s.err
}

func refreshServer(service refresh.Refresher) *Server {
	server := NewServer(nil, "test", nil)
	server.SetRefreshService(service)
	return server
}

func TestRF005_SinCookieDevuelve401(t *testing.T) {
	recorder := httptest.NewRecorder()
	refreshServer(&refreshStub{}).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "" || cookies[0].MaxAge >= 0 {
		t.Fatalf("cookies=%+v, want the compromised cookie cleared even without one presented", cookies)
	}
}

func TestRF005_RenovarEmiteParDistintoEInvalidaElAnterior(t *testing.T) {
	service := &refreshStub{result: refresh.Result{AccessToken: "access", RefreshToken: "new-refresh", TokenType: "Bearer", ExpiresIn: 900}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "old-refresh"})
	recorder := httptest.NewRecorder()
	refreshServer(service).Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.input.RefreshToken != "old-refresh" {
		t.Fatalf("input=%+v, want the cookie value bound by the generated router", service.input)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value == "old-refresh" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookies=%+v", cookies)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["accessToken"] != "access" || body["refreshToken"] != nil {
		t.Fatalf("body=%v", body)
	}
}

func TestRF006_ReusoDeTokenRotadoRevocaLaFamilia(t *testing.T) {
	service := &refreshStub{err: refresh.ErrRefreshReuse}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "reused"})
	recorder := httptest.NewRecorder()
	refreshServer(service).Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 || !cookies[0].Expires.Before(time.Now()) {
		t.Fatalf("cookies=%+v", cookies)
	}
}

func TestRF005_RefreshHandlerRechazaErroresNoAutorizados(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "bad"})
	recorder := httptest.NewRecorder()
	refreshServer(&refreshStub{err: errors.New("database")}).Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recorder.Code)
	}
}
