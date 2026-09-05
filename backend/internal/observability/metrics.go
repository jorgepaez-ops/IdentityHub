package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Métricas del dominio de seguridad. Son las que alimentan el panel de Grafana
// descrito en RNF-007; las de proceso y runtime las aporta el colector por
// defecto de client_golang.
var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "identity_http_requests_total",
		Help: "Peticiones HTTP atendidas, por ruta, método y código.",
	}, []string{"route", "method", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "identity_http_request_duration_seconds",
		Help:    "Latencia de las peticiones HTTP.",
		Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"route", "method"})

	LoginAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "identity_login_attempts_total",
		Help: "Intentos de inicio de sesión, por resultado.",
	}, []string{"result"}) // succeeded | failed | locked | mfa_required

	RefreshReuseDetected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "identity_refresh_reuse_detected_total",
		Help: "Reusos de refresh token detectados; cada uno revoca una familia de sesiones (RF-006).",
	})

	EventsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "identity_events_published_total",
		Help: "Eventos publicados en el broker, por tipo y resultado.",
	}, []string{"event_type", "result"})

	EventsConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "identity_events_consumed_total",
		Help: "Eventos consumidos por el worker, por tipo y resultado.",
	}, []string{"event_type", "result"})
)

// MetricsHandler expone el endpoint /metrics. Vive aquí para que la API y el
// worker no dupliquen la importación de promhttp.
func MetricsHandler() http.Handler { return promhttp.Handler() }
