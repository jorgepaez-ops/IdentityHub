package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The deferred endpoints must stay explicitly visible to clients as RFC 7807
// 501 responses. Exercising the generated routes keeps that contract from
// silently becoming a 404 while their domain implementation is pending.
func TestRNF005_DeferredContractOperationsReturnProblemNotImplemented(t *testing.T) {
	sessionID := uuid.New()
	for _, tt := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "verify MFA", method: http.MethodPost, path: "/api/v1/auth/mfa/verify"},
		{name: "confirm password reset", method: http.MethodPost, path: "/api/v1/auth/password-reset/confirm"},
		{name: "request password reset", method: http.MethodPost, path: "/api/v1/auth/password-reset/request"},
		{name: "disable MFA", method: http.MethodDelete, path: "/api/v1/me/mfa"},
		{name: "activate MFA", method: http.MethodPost, path: "/api/v1/me/mfa/activate"},
		{name: "enroll MFA", method: http.MethodPost, path: "/api/v1/me/mfa/enroll"},
		{name: "list sessions", method: http.MethodGet, path: "/api/v1/me/sessions"},
		{name: "revoke session", method: http.MethodDelete, path: "/api/v1/me/sessions/" + sessionID.String()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			NewServer(nil, "test", nil).Routes().ServeHTTP(response, request)
			if response.Code != http.StatusNotImplemented {
				t.Fatalf("status=%d body=%s, want 501", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
				t.Fatalf("Content-Type=%q, want application/problem+json", contentType)
			}
			if !strings.Contains(response.Body.String(), `"type":"https://identity.local/problems/not-implemented"`) {
				t.Fatalf("body=%s, want not-implemented problem type", response.Body.String())
			}
		})
	}
}
