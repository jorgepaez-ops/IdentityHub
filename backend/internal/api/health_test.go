package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type checkerStub struct{ err error }

func (c checkerStub) Ping(context.Context) error { return c.err }

func TestRNF012_ReadinessNoFiltraDetalleDeDependencia(t *testing.T) {
	var logs bytes.Buffer
	internalDetail := "postgres://identity_user@db.internal:5432/identity: connection refused"
	server := NewServer(slog.New(slog.NewTextHandler(&logs, nil)), "test", map[string]Checker{
		"database": checkerStub{err: errors.New(internalDetail)},
		"broker":   checkerStub{},
	})
	response := httptest.NewRecorder()

	server.Readiness(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if strings.Contains(response.Body.String(), internalDetail) {
		t.Fatalf("readiness leaked dependency detail: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"down"`) || !strings.Contains(response.Body.String(), `"status":"up"`) {
		t.Fatalf("body=%s, want generic per-dependency status", response.Body.String())
	}
	if !strings.Contains(logs.String(), internalDetail) || !strings.Contains(logs.String(), "database") {
		t.Fatalf("log=%q, want dependency name and complete error", logs.String())
	}
}
