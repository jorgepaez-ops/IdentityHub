package events

import (
	"encoding/json"
	"testing"
	"time"
)

// RF-012 — Cada evento necesita un identificador único: es lo que permite al
// worker ser idempotente frente a la entrega "al menos una vez" de RabbitMQ.
// Sin él, un reintento del broker se traduce en un segundo correo.
func TestRF012_CadaEnvelopeLlevaUnIdentificadorUnico(t *testing.T) {
	vistos := make(map[string]bool, 1000)

	for i := 0; i < 1000; i++ {
		e := NewEnvelope(TypeUserRegistered, "traza-1")

		id := e.EventID.String()
		if vistos[id] {
			t.Fatalf("eventId repetido en la iteración %d: %s", i, id)
		}
		vistos[id] = true

		if e.EventType != TypeUserRegistered {
			t.Errorf("EventType = %q; se esperaba %q", e.EventType, TypeUserRegistered)
		}
		if e.Version != 1 {
			t.Errorf("Version = %d; se esperaba 1", e.Version)
		}
		if e.OccurredAt.Location() != time.UTC {
			t.Errorf("OccurredAt debe estar en UTC para poder comparar entre servicios; está en %v", e.OccurredAt.Location())
		}
	}
}

// El sobre serializado tiene que usar exactamente los nombres de campo que
// declara specs/04-events/asyncapi.yaml. Un desajuste aquí rompe al consumidor
// en tiempo de ejecución, no de compilación, así que se comprueba en una prueba.
func TestRF012_ElSobreSerializadoRespetaElContrato(t *testing.T) {
	e := NewEnvelope(TypeRefreshReuseDetected, "traza-2")

	crudo, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var campos map[string]any
	if err := json.Unmarshal(crudo, &campos); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// Nombres tomados del componente Envelope del AsyncAPI.
	for _, obligatorio := range []string{"eventId", "eventType", "occurredAt", "version"} {
		if _, ok := campos[obligatorio]; !ok {
			t.Errorf("falta el campo %q exigido por specs/04-events/asyncapi.yaml; encontrados: %v", obligatorio, claves(campos))
		}
	}
}

// Las claves de enrutado tienen que coincidir literalmente con los canales del
// AsyncAPI, porque son la routing key del exchange topic.
func TestRF012_LosTiposDeEventoCoincidenConLosCanalesDelSpec(t *testing.T) {
	esperados := map[string]string{
		TypeUserRegistered:         "user.registered",
		TypeEmailVerified:          "user.email_verified",
		TypePasswordResetRequested: "user.password_reset_requested",
		TypeRefreshReuseDetected:   "security.refresh_reuse_detected",
		TypeAccountLocked:          "security.account_locked",
	}

	for constante, canal := range esperados {
		if constante != canal {
			t.Errorf("la constante vale %q pero el canal del spec es %q", constante, canal)
		}
	}

	// La cola `notifications` está enlazada a `user.*` y `security.*`: todo tipo
	// de evento tiene que caer bajo uno de esos dos prefijos o se perdería en
	// silencio, que es la peor forma de perder un mensaje.
	for tipo := range esperados {
		if len(tipo) < 6 || (tipo[:5] != "user." && tipo[:9] != "security.") {
			t.Errorf("el tipo %q no encaja con los enlaces user.* ni security.* de la cola %s", tipo, QueueNotify)
		}
	}
}

func claves(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
