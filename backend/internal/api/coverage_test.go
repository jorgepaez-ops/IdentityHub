package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Implemented endpoints must remain visible through generated routes even
// before dependency injection finishes composition.
func TestRNF005_OAuthTokenWithoutDependenciesReturnsProblem(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "exchange authorization code", method: http.MethodPost, path: "/oauth/token"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			NewServer(nil, "test", nil).Routes().ServeHTTP(response, request)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s, want 503", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
				t.Fatalf("Content-Type=%q, want application/problem+json", contentType)
			}
			if !strings.Contains(response.Body.String(), `"type":"https://identity.local/problems/oauth-unavailable"`) {
				t.Fatalf("body=%s, want oauth-unavailable problem type", response.Body.String())
			}
		})
	}
}
