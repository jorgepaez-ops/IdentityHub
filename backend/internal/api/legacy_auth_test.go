package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRF004_LaRutaLegacyYaNoExiste confirma que el endpoint de la línea base
// vulnerable (VULN-001, 002, 004, 005, 006, 007) ya no está expuesto por el
// router real: T23 elimina legacy_auth.go entero (ver adr/0007 y
// odd/tasks/idp-semana-2.md).
func TestRF004_LaRutaLegacyYaNoExiste(t *testing.T) {
	server := NewServer(nil, "test", nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/legacy-login", nil)
	recorder := httptest.NewRecorder()

	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
