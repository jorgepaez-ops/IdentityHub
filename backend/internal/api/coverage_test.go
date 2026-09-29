package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The deferred endpoints must stay explicitly visible to clients as RFC 7807
// 501 responses. Exercising the generated routes keeps that contract from
// silently becoming a 404 while their domain implementation is pending.
func TestRNF005_DeferredContractOperationsReturnProblemNotImplemented(t *testing.T) {
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
