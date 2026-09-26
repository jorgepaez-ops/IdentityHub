// Binario de la API: el proveedor de identidad.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/api"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/logout"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	"github.com/jorgepaez/identity-hub/internal/auth/registration"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/auth/verification"
	"github.com/jorgepaez/identity-hub/internal/config"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/observability"
	"github.com/jorgepaez/identity-hub/internal/store"
)

func main() {
	// The distroless image (T27) has no shell or curl: the compose healthcheck
	// invokes the binary itself instead of an external process.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck(http.DefaultClient, healthcheckURL()))
	}
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "la api no pudo arrancar: %v\n", err)
		os.Exit(1)
	}
}

// healthcheckURL reads API_PORT directly instead of going through
// config.Load(), which requires every mandatory variable of the main
// process: the healthcheck only needs to know which local port to ask.
func healthcheckURL() string {
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8081"
	}
	return fmt.Sprintf("http://127.0.0.1:%s/healthz", port)
}

func runHealthcheck(client *http.Client, url string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 1
	}
	resp, err := client.Do(req)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
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

	seed, err := base64.StdEncoding.DecodeString(cfg.JWTSigningKey.Reveal())
	if err != nil {
		return fmt.Errorf("decode JWT signing key: %w", err)
	}
	tokens, err := token.New(seed, cfg.JWTIssuer, cfg.JWTAudience, time.Now)
	if err != nil {
		return fmt.Errorf("create token service: %w", err)
	}
	password.Configure(cfg.Argon2)

	registrationService := registration.New(db, broker, passwordHasher{}, rand.Reader, time.Now)
	verificationService := verification.New(db, broker)
	loginService := login.New(db, tokens, cfg.RefreshTTL, login.LockoutConfig{
		AccountMaxFailures: cfg.LoginAccountMaxFailures,
		IPMaxFailures:      cfg.LoginIPMaxFailures,
		FailureWindow:      cfg.LoginFailureWindow,
		LockoutDuration:    cfg.LoginLockoutDuration,
	}).WithEventPublisher(loginSecurityEventPublisher{users: db, publisher: broker, logger: logger})
	refreshService := refresh.New(db, tokens, cfg.RefreshTTL).WithEventPublisher(refreshSecurityEventPublisher{users: db, publisher: broker, logger: logger})

	server := api.NewServer(logger, cfg.Version, map[string]api.Checker{
		"database": db,
		"broker":   brokerChecker{broker},
	})
	server.SetTokenService(tokens)
	server.SetRegistrationService(registrationService)
	server.SetLoginService(loginService, cfg.RefreshTTL)
	server.SetEmailVerificationService(verificationService)
	server.SetRefreshService(refreshService)
	server.SetLogoutService(logout.New(db))
	server.SetAdminUserService(admin.New(db))
	server.SetAuditLogService(auditlog.New(db))
	server.SetCurrentUserRepository(db)
	server.SetTrustedProxies(cfg.TrustedProxies)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           server.Routes(),
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

type passwordHasher struct{}

func (passwordHasher) Hash(value string) (string, error) { return password.Hash(value) }

type userLookup interface {
	GetUserByID(context.Context, uuid.UUID) (store.User, error)
}

type eventPublisher interface {
	Publish(context.Context, string, any) error
}

// loginSecurityEventPublisher enriches committed lockout events with the recipient
// data required by the notification worker. A missing user is non-fatal because the
// lockout transaction has already committed.
type loginSecurityEventPublisher struct {
	users     userLookup
	publisher eventPublisher
	logger    *slog.Logger
}

func (p loginSecurityEventPublisher) PublishSecurityEvent(ctx context.Context, event login.SecurityEvent) error {
	user, ok := p.lookupUser(ctx, event.UserID, event.Type)
	if !ok {
		return nil
	}
	message := accountLockedNotification{Envelope: events.NewEnvelope(events.TypeAccountLocked, api.TraceIDFrom(ctx))}
	message.Data.UserID = event.UserID
	message.Data.Email = user.Email
	message.Data.DisplayName = user.DisplayName
	message.Data.LockedUntil = event.LockedUntil
	message.Data.FailedAttempts = event.FailedAttempts
	message.Data.IP = addressString(event.IP)
	return p.publisher.Publish(ctx, events.TypeAccountLocked, message)
}

func (p loginSecurityEventPublisher) lookupUser(ctx context.Context, userID uuid.UUID, eventType string) (store.User, bool) {
	user, err := p.users.GetUserByID(ctx, userID)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("security event skipped because recipient could not be loaded", "event_type", eventType, "user_id", userID, "error", err)
		}
		return store.User{}, false
	}
	return user, true
}

// refreshSecurityEventPublisher is deliberately separate from the login adapter:
// the two services expose different SecurityEvent types despite sharing delivery.
type refreshSecurityEventPublisher struct {
	users     userLookup
	publisher eventPublisher
	logger    *slog.Logger
}

func (p refreshSecurityEventPublisher) PublishSecurityEvent(ctx context.Context, event refresh.SecurityEvent) error {
	user, err := p.users.GetUserByID(ctx, event.UserID)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("security event skipped because recipient could not be loaded", "event_type", event.Type, "user_id", event.UserID, "error", err)
		}
		return nil
	}
	message := refreshReuseNotification{Envelope: events.NewEnvelope(events.TypeRefreshReuseDetected, api.TraceIDFrom(ctx))}
	message.Data.UserID = event.UserID
	message.Data.Email = user.Email
	message.Data.DisplayName = user.DisplayName
	message.Data.FamilyID = event.FamilyID
	message.Data.RevokedCount = int(event.RevokedCount)
	message.Data.IP = addressString(event.IP)
	return p.publisher.Publish(ctx, events.TypeRefreshReuseDetected, message)
}

type accountLockedNotification struct {
	events.Envelope
	Data struct {
		UserID         uuid.UUID `json:"userId"`
		Email          string    `json:"email"`
		DisplayName    string    `json:"displayName"`
		LockedUntil    time.Time `json:"lockedUntil"`
		FailedAttempts int       `json:"failedAttempts"`
		IP             string    `json:"ip"`
	} `json:"data"`
}

type refreshReuseNotification struct {
	events.Envelope
	Data struct {
		UserID       uuid.UUID `json:"userId"`
		Email        string    `json:"email"`
		DisplayName  string    `json:"displayName"`
		FamilyID     uuid.UUID `json:"familyId"`
		RevokedCount int       `json:"revokedCount"`
		IP           string    `json:"ip"`
	} `json:"data"`
}

func addressString(address *netip.Addr) string {
	if address == nil {
		return ""
	}
	return address.String()
}
