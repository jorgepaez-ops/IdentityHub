package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jorgepaez/identity-hub/internal/observability"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Metrics instrumenta cada petición. Usa el patrón de ruta de chi
// (`/api/v1/admin/users/{userId}`) y no la URL concreta: etiquetar con el path
// real haría explotar la cardinalidad de Prometheus con un UUID por serie.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unknown"
		}
		observability.HTTPDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
		observability.HTTPRequests.WithLabelValues(route, r.Method, http.StatusText(rec.status)).Inc()
	})
}

type ctxKey string

const traceIDKey ctxKey = "traceID"

// TraceID propaga un identificador de correlación entre los logs de la API y
// los eventos que publica, para poder seguir un registro desde la petición HTTP
// hasta el correo entregado por el worker.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(withTraceID(r.Context(), id)))
	})
}
