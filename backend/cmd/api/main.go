// Binario de la API: el proveedor de identidad.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jorgepaez/identity-hub/internal/api"
	"github.com/jorgepaez/identity-hub/internal/config"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/observability"
	"github.com/jorgepaez/identity-hub/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "la api no pudo arrancar: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := observability.NewLogger(cfg.LogLevel, "api", cfg.Version)
	logger.Info("arrancando", "port", cfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.New(ctx, cfg.DatabaseURL.Reveal())
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("conectado a postgres")

	broker, err := events.Connect(cfg.RabbitURL.Reveal())
	if err != nil {
		return err
	}
	defer broker.Close()
	logger.Info("conectado al broker y topología declarada")

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: api.NewServer(logger, cfg.Version, map[string]api.Checker{
			"database": db,
			"broker":   brokerChecker{broker},
		}).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	logger.Info("escuchando", "addr", srv.Addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("señal recibida, cerrando ordenadamente")
	}

	// Margen para que las peticiones en vuelo terminen antes de cortar. Sin
	// esto, un despliegue devuelve 502 a quien estuviera a mitad de un login.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// brokerChecker adapta el broker a la interfaz Checker de /readyz, que recibe
// un contexto que la comprobación de AMQP no necesita.
type brokerChecker struct{ b *events.Broker }

func (c brokerChecker) Ping(_ context.Context) error { return c.b.Ping() }
