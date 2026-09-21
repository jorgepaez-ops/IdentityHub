package api

import (
	"context"
	"encoding/json"
	"net/http"
)

// Checker es cualquier dependencia cuya salud se refleja en /readyz.
type Checker interface {
	Ping(ctx context.Context) error
}

type checkResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type readiness struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks"`
}

// Health responde la sonda de vitalidad: ¿está vivo el proceso?
// Deliberadamente no consulta servicios externos — si lo hiciera, una base de datos
// caída provocaría que el orquestador reiniciara la API en bucle sin motivo.
func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": s.version,
	})
}

// Readiness responde la sonda de disponibilidad: ¿puede atender tráfico?
// Aquí sí se consultan los servicios externos; un fallo devuelve 503 y saca la
// instancia de rotación sin matarla.
func (s *Server) Readiness(w http.ResponseWriter, r *http.Request) {
	out := readiness{Status: "ready", Checks: map[string]checkResult{}}

	for name, dep := range s.deps {
		if err := dep.Ping(r.Context()); err != nil {
			out.Status = "degraded"
			out.Checks[name] = checkResult{Status: "down", Error: err.Error()}
			continue
		}
		out.Checks[name] = checkResult{Status: "up"}
	}

	code := http.StatusOK
	if out.Status != "ready" {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, out)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// writeProblem emite un error en formato RFC 7807, tal y como lo declara el
// esquema Problem del OpenAPI.
func writeProblem(w http.ResponseWriter, status int, kind, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "https://identity.local/problems/" + kind,
		"title":  title,
		"status": status,
		"detail": detail,
	})
}
