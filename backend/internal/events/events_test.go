package events

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/observability"
	"github.com/prometheus/client_golang/prometheus/testutil"
	amqp "github.com/rabbitmq/amqp091-go"
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
		TypeUserInvited:            "user.invited",
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
		if !strings.HasPrefix(tipo, "user.") && !strings.HasPrefix(tipo, "security.") {
			t.Errorf("el tipo %q no encaja con los enlaces user.* ni security.* de la cola %s", tipo, QueueNotify)
		}
	}
}

// RNF-005 — Ping se usa para /readyz: un Broker sin conexión (por ejemplo, si
// Connect nunca llegó a asignarla) no debe reportarse como sano.
func TestRNF005_PingFallaConConexionNula(t *testing.T) {
	b := &Broker{}

	if err := b.Ping(); err == nil {
		t.Fatal("Ping() = nil; se esperaba un error con conn == nil")
	}
}

// Close no debe entrar en pánico aunque Connect nunca haya llegado a asignar
// canal o conexión: un fallo a mitad de Connect ya llama a Close() sobre un
// Broker parcialmente construido.
func TestRNF005_CloseNoEntraEnPanicoConBrokerVacio(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close() entró en pánico con un Broker vacío: %v", r)
		}
	}()

	(&Broker{}).Close()
}

// Channel expone el canal AMQP subyacente tal cual, sin envolverlo: lo usa
// cmd/worker para registrar el consumidor.
func TestRNF005_ChannelDevuelveNilSinCanalAsignado(t *testing.T) {
	b := &Broker{}

	if got := b.Channel(); got != nil {
		t.Fatalf("Channel() = %v; se esperaba nil sobre un Broker sin canal asignado", got)
	}
}

// Connect debe envolver el error de conexión, no perderlo ni entrar en pánico,
// cuando el broker es inalcanzable.
func TestRNF005_ConnectEnvuelveElErrorDeConexion(t *testing.T) {
	_, err := Connect("amqp://127.0.0.1:1/")
	if err == nil {
		t.Fatal("Connect() = nil error; se esperaba un fallo de conexión contra un puerto inalcanzable")
	}
	const prefijo = "no se pudo conectar al broker: "
	if !strings.HasPrefix(err.Error(), prefijo) {
		t.Errorf("error = %q; se esperaba que empezara con %q (Connect debe envolver el error, no perderlo)", err.Error(), prefijo)
	}
}

// Publish serializa antes de tocar el canal: un evento no serializable debe
// fallar ahí mismo, sin necesitar una conexión real al broker.
func TestRNF005_PublishFallaAlSerializarSinNecesitarBroker(t *testing.T) {
	b := &Broker{}

	// Un canal de Go nunca es serializable a JSON.
	err := b.Publish(context.Background(), TypeUserRegistered, make(chan int))
	if err == nil {
		t.Fatal("Publish() = nil error; se esperaba un fallo de serialización")
	}
}

func TestRNF007_PublishCuentaFalloDeSerializacion(t *testing.T) {
	metric := observability.EventsPublished.WithLabelValues(TypeUserRegistered, observability.ResultFailed)
	before := testutil.ToFloat64(metric)

	err := (&Broker{}).Publish(context.Background(), TypeUserRegistered, make(chan int))
	if err == nil {
		t.Fatal("Publish() = nil error; se esperaba un fallo de serialización")
	}

	if got := testutil.ToFloat64(metric); got != before+1 {
		t.Fatalf("contador failed = %v; se esperaba %v", got, before+1)
	}
}

type publisherStub struct {
	confirmation deferredConfirmation
	err          error
}

func (p publisherStub) PublishWithDeferredConfirmWithContext(context.Context, string, string, bool, bool, amqp.Publishing) (deferredConfirmation, error) {
	return p.confirmation, p.err
}

type confirmationStub struct {
	confirmed bool
	err       error
}

func (c confirmationStub) WaitContext(context.Context) (bool, error) { return c.confirmed, c.err }

func TestRNF007_PublishCuentaResultadosConfirmadosYFallidos(t *testing.T) {
	const routingKey = TypeAccountLocked
	for _, testCase := range []struct {
		name    string
		event   any
		publish eventPublisher
		result  string
	}{
		{name: "serialización", event: make(chan int), result: observability.ResultFailed},
		{name: "publicación", event: NewEnvelope(routingKey, ""), publish: publisherStub{err: errors.New("broker unavailable")}, result: observability.ResultFailed},
		{name: "espera de confirmación", event: NewEnvelope(routingKey, ""), publish: publisherStub{confirmation: confirmationStub{err: errors.New("confirmation timeout")}}, result: observability.ResultFailed},
		{name: "nack", event: NewEnvelope(routingKey, ""), publish: publisherStub{confirmation: confirmationStub{}}, result: observability.ResultFailed},
		{name: "confirmación", event: NewEnvelope(routingKey, ""), publish: publisherStub{confirmation: confirmationStub{confirmed: true}}, result: observability.ResultPublished},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			metric := observability.EventsPublished.WithLabelValues(routingKey, testCase.result)
			before := testutil.ToFloat64(metric)

			err := (&Broker{publisher: testCase.publish}).Publish(context.Background(), routingKey, testCase.event)
			if testCase.result == observability.ResultPublished && err != nil {
				t.Fatalf("Publish() error = %v", err)
			}
			if testCase.result == observability.ResultFailed && err == nil {
				t.Fatal("Publish() = nil error; se esperaba un fallo")
			}
			if got := testutil.ToFloat64(metric); got != before+1 {
				t.Fatalf("contador %s = %v; se esperaba %v", testCase.result, got, before+1)
			}
		})
	}
}

func claves(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
