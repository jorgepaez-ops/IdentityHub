// Package observability concentra logs estructurados y métricas (RNF-007).
package observability

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger devuelve un logger JSON. El formato estructurado no es estética:
// es lo que permite que Loki indexe por campo y que Grafana grafique, por
// ejemplo, los logins fallidos por IP sin analizar texto libre.
func NewLogger(level, service, version string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
		// Normaliza la marca temporal a RFC3339 en UTC para que los logs de los
		// tres servicios sean comparables en Loki.
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().UTC().Format("2006-01-02T15:04:05.000Z"))
			}
			return a
		},
	})

	return slog.New(handler).With(
		slog.String("service", service),
		slog.String("version", version),
	)
}

// TokenRef acorta un identificador de token a 8 caracteres para poder
// correlacionarlo en los logs sin registrar la credencial (RNF-012 / AM-013).
func TokenRef(token string) string {
	if len(token) <= 8 {
		return "********"
	}
	return token[:8] + "…"
}
