package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

var _ ServerInterface = (*Server)(nil)

func TestRNF011_GeneratedServerContract(t *testing.T) {
	t.Run("health endpoint remains available", func(t *testing.T) {
		server := NewServer(nil, "test", nil)
		response := httptest.NewRecorder()

		server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		if response.Code != http.StatusOK {
			t.Fatalf("GET /healthz status = %d, want %d", response.Code, http.StatusOK)
		}
	})

	t.Run("refresh token is only a cookie parameter", func(t *testing.T) {
		if _, exists := reflect.TypeOf(TokenPair{}).FieldByName("RefreshToken"); exists {
			t.Fatal("TokenPair must not expose RefreshToken")
		}

		contract := reflect.TypeOf((*ServerInterface)(nil)).Elem()
		if contract.NumMethod() != 28 {
			t.Fatalf("ServerInterface methods = %d, want 28", contract.NumMethod())
		}
		for _, operation := range []string{"RefreshSession", "Logout"} {
			method, ok := contract.MethodByName(operation)
			if !ok {
				t.Fatalf("ServerInterface is missing %s", operation)
			}
			if method.Type.NumIn() != 3 {
				t.Fatalf("%s parameters = %d, want response writer, request, and cookie params", operation, method.Type.NumIn())
			}
			params := method.Type.In(2)
			field, ok := params.FieldByName("RefreshToken")
			if !ok || field.Tag.Get("form") != "refresh_token" {
				t.Fatalf("%s must accept refresh_token as a cookie parameter", operation)
			}
		}
	})
}

func TestRNF012_ErrorDeBindingEsProblemJSONSinDetalleInterno(t *testing.T) {
	server := NewServer(nil, "test", nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	internalDetail := "dial tcp db.internal:5432: connection refused"

	server.handleBindingError(response, request, errors.New(internalDetail))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
		t.Fatalf("Content-Type = %q, want application/problem+json", contentType)
	}
	if strings.Contains(response.Body.String(), internalDetail) {
		t.Fatalf("response leaked binding detail: %s", response.Body.String())
	}
	var problem map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode RFC 7807 response: %v", err)
	}
	if problem["status"] != float64(http.StatusBadRequest) || problem["title"] == "" || problem["detail"] == "" {
		t.Fatalf("problem = %#v, want populated RFC 7807 fields", problem)
	}
}
