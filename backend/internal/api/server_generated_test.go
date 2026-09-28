package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
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
		if contract.NumMethod() != 23 {
			t.Fatalf("ServerInterface methods = %d, want 23", contract.NumMethod())
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
