package observability

import (
	"strings"
	"testing"
)

// RNF-012 / AM-013 — Para investigar un incidente hace falta poder correlacionar
// tokens en los logs, pero registrar el token entero convierte el log en una
// fuente de credenciales. TokenRef resuelve las dos cosas: identifica sin
// revelar.
func TestRNF012_TokenRefNoRevelaElToken(t *testing.T) {
	casos := []struct {
		nombre     string
		token      string
		esperado   string
		noContiene string
	}{
		{
			nombre:     "token largo: prefijo y elipsis",
			token:      "aGVsbG8td29ybGQtZXN0ZS1lcy11bi1yZWZyZXNoLXRva2Vu",
			esperado:   "aGVsbG8t…",
			noContiene: "d29ybGQ",
		},
		{
			nombre:   "token corto: se enmascara entero",
			token:    "abc123",
			esperado: "********",
		},
		{
			nombre:   "cadena vacía",
			token:    "",
			esperado: "********",
		},
		{
			nombre:   "exactamente 8 caracteres: se enmascara, no se revela entero",
			token:    "12345678",
			esperado: "********",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := TokenRef(c.token)
			if got != c.esperado {
				t.Errorf("TokenRef(%q) = %q; se esperaba %q", c.token, got, c.esperado)
			}
			if c.noContiene != "" && strings.Contains(got, c.noContiene) {
				t.Errorf("TokenRef filtró parte del token: %q", got)
			}
		})
	}
}

func TestNewLogger_InterpretaElNivel(t *testing.T) {
	for _, nivel := range []string{"debug", "info", "warn", "error", "DEBUG", "basura"} {
		if l := NewLogger(nivel, "test", "0.0.0"); l == nil {
			t.Errorf("NewLogger(%q) devolvió nil; un nivel desconocido debe caer en info, no fallar", nivel)
		}
	}
}
