// Binario del worker: consume eventos del broker y entrega notificaciones por
// SMTP (RF-012).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jorgepaez/identity-hub/internal/config"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/observability"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "el worker no pudo arrancar: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := observability.NewLogger(cfg.LogLevel, "worker", cfg.Version)
	logger.Info("arrancando")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	broker, err := events.Connect(cfg.RabbitURL.Reveal())
	if err != nil {
		return err
	}
	defer broker.Close()
	logger.Info("conectado al broker")

	// Prefetch de 10: sin él, RabbitMQ empuja la cola entera a un solo
	// consumidor y el reparto entre réplicas deja de existir.
	if err := broker.Channel().Qos(10, 0, false); err != nil {
		return fmt.Errorf("configurando el prefetch: %w", err)
	}

	deliveries, err := broker.Channel().Consume(events.QueueNotify, "worker", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumiendo de %s: %w", events.QueueNotify, err)
	}

	// El worker expone /metrics y /healthz en su propio puerto para que
	// Prometheus lo raspe igual que a la API.
	metricsSrv := startMetricsServer(logger, cfg.Version)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	w := &worker{logger: logger, cfg: cfg, seen: make(map[string]time.Time)}
	logger.Info("esperando eventos", "queue", events.QueueNotify)

	for {
		select {
		case <-ctx.Done():
			logger.Info("señal recibida, cerrando ordenadamente")
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("el canal de entregas se cerró; ¿se cayó el broker?")
			}
			w.handle(ctx, d)
		}
	}
}

type worker struct {
	logger *slog.Logger
	cfg    *config.Config

	// Idempotencia (specs/04-events/topology.md): RabbitMQ garantiza entrega
	// "al menos una vez", así que un reintento no debe traducirse en un segundo
	// correo. En memoria basta para un worker único; con varias réplicas esto
	// tendría que vivir en Postgres o Redis.
	mu   sync.Mutex
	seen map[string]time.Time
}

func (w *worker) alreadyProcessed(eventID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Purga perezosa: los identificadores de más de una hora ya no pueden
	// reaparecer, porque x-delivery-limit corta a los 3 intentos.
	cutoff := time.Now().Add(-time.Hour)
	for id, at := range w.seen {
		if at.Before(cutoff) {
			delete(w.seen, id)
		}
	}

	if _, dup := w.seen[eventID]; dup {
		return true
	}
	w.seen[eventID] = time.Now()
	return false
}

func (w *worker) handle(ctx context.Context, d amqp.Delivery) {
	var env events.Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		// Un mensaje ilegible no mejora reintentándolo: va directo a la DLQ.
		w.logger.Error("evento ilegible, enviado a la DLQ", "error", err)
		observability.EventsConsumed.WithLabelValues("unknown", "malformed").Inc()
		_ = d.Reject(false)
		return
	}

	log := w.logger.With("event_type", env.EventType, "event_id", env.EventID.String(), "trace_id", env.TraceID)

	if w.alreadyProcessed(env.EventID.String()) {
		log.Info("evento duplicado, descartado sin reenviar")
		observability.EventsConsumed.WithLabelValues(env.EventType, "duplicate").Inc()
		_ = d.Ack(false)
		return
	}

	// Nunca se registra d.Body: los eventos user.registered y
	// password_reset_requested llevan tokens en claro (RNF-012 / AM-016).
	if err := w.deliver(ctx, env, d.Body); err != nil {
		log.Error("no se pudo entregar la notificación", "error", err)
		observability.EventsConsumed.WithLabelValues(env.EventType, "failed").Inc()
		// Nack con requeue: el broker reintenta hasta x-delivery-limit y
		// después lo deriva solo a la DLQ.
		_ = d.Nack(false, true)
		return
	}

	log.Info("notificación entregada")
	observability.EventsConsumed.WithLabelValues(env.EventType, "delivered").Inc()
	_ = d.Ack(false)
}

// deliver arma y envía el correo. En la semana 2 se sustituye por plantillas
// por tipo de evento; aquí basta con demostrar el recorrido completo
// broker → worker → SMTP → Mailpit.
func (w *worker) deliver(_ context.Context, env events.Envelope, body []byte) error {
	var payload struct {
		Data struct {
			Email       string `json:"email"`
			DisplayName string `json:"displayName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("extrayendo el destinatario: %w", err)
	}
	if payload.Data.Email == "" {
		return fmt.Errorf("el evento %s no trae destinatario", env.EventType)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: [Identity Hub] %s\r\n\r\n"+
		"Hola %s:\r\n\r\nEvento: %s\r\nFecha: %s\r\n",
		w.cfg.SMTPFrom, payload.Data.Email, env.EventType,
		payload.Data.DisplayName, env.EventType, env.OccurredAt.Format(time.RFC3339))

	addr := net.JoinHostPort(w.cfg.SMTPHost, fmt.Sprint(w.cfg.SMTPPort))
	// Mailpit no exige autenticación ni TLS; en un entorno real aquí irían
	// credenciales y STARTTLS.
	return smtp.SendMail(addr, nil, w.cfg.SMTPFrom, []string{payload.Data.Email}, []byte(msg))
}

func startMetricsServer(logger *slog.Logger, version string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", observability.MetricsHandler())
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(rw, `{"status":"ok","version":%q}`, version)
	})

	srv := &http.Server{Addr: ":9091", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("el servidor de métricas se detuvo", "error", err)
		}
	}()
	return srv
}
