package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Broker mantiene la conexión y el canal con RabbitMQ y declara la topología.
type Broker struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// Connect abre la conexión y declara exchanges y colas de forma idempotente.
// Declarar desde ambos extremos (API y worker) evita depender del orden de
// arranque de los contenedores.
func Connect(url string) (*Broker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar al broker: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("no se pudo abrir el canal: %w", err)
	}

	// Publisher confirms: sin esto, Publish() solo escribe en un socket y
	// devuelve nil aunque el broker nunca haya recibido nada (ver adr/0006).
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("no se pudo activar publisher confirms: %w", err)
	}

	b := &Broker{conn: conn, channel: ch}
	if err := b.declareTopology(); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

func (b *Broker) declareTopology() error {
	if err := b.channel.ExchangeDeclare(ExchangeEvents, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarando el exchange %s: %w", ExchangeEvents, err)
	}
	if err := b.channel.ExchangeDeclare(ExchangeDLX, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarando el exchange %s: %w", ExchangeDLX, err)
	}

	if _, err := b.channel.QueueDeclare(QueueNotify, true, false, false, false, amqp.Table{
		"x-queue-type":           "quorum",
		"x-dead-letter-exchange": ExchangeDLX,
		"x-delivery-limit":       int32(3),
	}); err != nil {
		return fmt.Errorf("declarando la cola %s: %w", QueueNotify, err)
	}
	if _, err := b.channel.QueueDeclare(QueueDLQ, true, false, false, false, amqp.Table{
		"x-queue-type": "quorum",
	}); err != nil {
		return fmt.Errorf("declarando la cola %s: %w", QueueDLQ, err)
	}

	// El worker de notificaciones escucha tanto los eventos de usuario como los
	// de seguridad: ambos terminan en un correo.
	for _, key := range []string{"user.*", "security.*"} {
		if err := b.channel.QueueBind(QueueNotify, key, ExchangeEvents, false, nil); err != nil {
			return fmt.Errorf("enlazando %s a %s: %w", QueueNotify, key, err)
		}
	}
	if err := b.channel.QueueBind(QueueDLQ, "", ExchangeDLX, false, nil); err != nil {
		return fmt.Errorf("enlazando la DLQ: %w", err)
	}
	return nil
}

// Publish serializa el evento y espera la confirmación del broker antes de
// devolver. Si el broker no confirma, el llamante debe revertir su transacción.
func (b *Broker) Publish(ctx context.Context, routingKey string, event any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("serializando el evento: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	confirm, err := b.channel.PublishWithDeferredConfirmWithContext(ctx,
		ExchangeEvents, routingKey, true, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		})
	if err != nil {
		return fmt.Errorf("publicando %s: %w", routingKey, err)
	}

	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("esperando confirmación de %s: %w", routingKey, err)
	}
	if !ok {
		return fmt.Errorf("el broker rechazó el evento %s", routingKey)
	}
	return nil
}

func (b *Broker) Channel() *amqp.Channel { return b.channel }

// Ping alimenta la sonda /readyz: una conexión cerrada por el broker no siempre
// se detecta hasta el primer uso.
func (b *Broker) Ping() error {
	if b.conn == nil || b.conn.IsClosed() {
		return fmt.Errorf("la conexión con el broker está cerrada")
	}
	return nil
}

func (b *Broker) Close() {
	if b.channel != nil {
		_ = b.channel.Close()
	}
	if b.conn != nil {
		_ = b.conn.Close()
	}
}
