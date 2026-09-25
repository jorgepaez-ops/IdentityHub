// Package store encapsula el acceso a PostgreSQL.
//
// A partir de la semana 2 las consultas se generan con sqlc desde
// db/migrations y db/queries. Este archivo solo mantiene el pool y las sondas.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

type Store struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// CreateUserParams contains the attributes accepted by the user creation
// adapter. It deliberately does not expose generated sqlc types to callers.
type CreateUserParams struct {
	Email        string
	PasswordHash string
	DisplayName  string
}

// User is the application-facing representation returned by base user
// lookups. Additional persistence-only fields remain inside the sqlc package.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	Status       string
}

// InsertAuditEventParams contains the fields accepted when appending an audit
// event. Nil pointer fields are stored as SQL NULL; Metadata must contain JSON.
type InsertAuditEventParams struct {
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType *string
	ResourceID   *string
	IP           *netip.Addr
	UserAgent    *string
	Metadata     json.RawMessage
}

// AuditEvent is the application-facing result of appending an audit event.
type AuditEvent struct {
	ID           int64
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType *string
	ResourceID   *string
	IP           *netip.Addr
	UserAgent    *string
	Metadata     json.RawMessage
}

// New abre el pool y verifica la conexión antes de devolver. Fallar aquí es
// mejor que descubrir la base de datos caída en la primera petición real.
func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DSN de PostgreSQL inválido: %w", err)
	}

	// Argon2id reserva 64 MiB por verificación (ver adr/0004), así que la
	// concurrencia útil de la API está acotada por memoria, no por la base de
	// datos. Un pool grande solo serviría para acumular trabajo en cola.
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear el pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("la base de datos no responde: %w", err)
	}

	return NewWithPool(pool)
}

// NewWithPool wraps an already-initialized pool. It is useful for callers that
// own the connection lifecycle, including integration tests with temporary
// databases.
func NewWithPool(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &Store{pool: pool, queries: generated.New(pool)}, nil
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// CreateUser persists a user through the generated parameterized query.
func (s *Store) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	user, err := s.queries.CreateUser(ctx, generated.CreateUserParams{
		Email:        params.Email,
		PasswordHash: params.PasswordHash,
		DisplayName:  params.DisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return userFromGenerated(user), nil
}

// GetUserByEmail retrieves one user. Its returned error preserves pgx.ErrNoRows
// so callers can distinguish an absent user without relying on generated types.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return userFromGenerated(user), nil
}

// GetUserByID retrieves one user by its UUID.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	user, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return userFromGenerated(user), nil
}

// InsertAuditEvent appends a fully parameterized audit event through the
// generated query.
func (s *Store) InsertAuditEvent(ctx context.Context, params InsertAuditEventParams) (AuditEvent, error) {
	event, err := s.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{
		ActorUserID:  nullableUUID(params.ActorUserID),
		Action:       params.Action,
		ResourceType: nullableText(params.ResourceType),
		ResourceID:   nullableText(params.ResourceID),
		Ip:           copyAddr(params.IP),
		UserAgent:    nullableText(params.UserAgent),
		Metadata:     params.Metadata,
	})
	if err != nil {
		return AuditEvent{}, fmt.Errorf("insert audit event: %w", err)
	}
	return auditEventFromGenerated(event), nil
}

func userFromGenerated(user generated.User) User {
	return User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		DisplayName:  user.DisplayName,
		Status:       string(user.Status),
	}
}

func auditEventFromGenerated(event generated.AuditLog) AuditEvent {
	return AuditEvent{
		ID:           event.ID,
		ActorUserID:  optionalUUID(event.ActorUserID),
		Action:       event.Action,
		ResourceType: optionalText(event.ResourceType),
		ResourceID:   optionalText(event.ResourceID),
		IP:           copyAddr(event.Ip),
		UserAgent:    optionalText(event.UserAgent),
		Metadata:     append(json.RawMessage(nil), event.Metadata...),
	}
}

func nullableUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func optionalUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	result := uuid.UUID(id.Bytes)
	return &result
}

func nullableText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func optionalText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func copyAddr(addr *netip.Addr) *netip.Addr {
	if addr == nil {
		return nil
	}
	result := *addr
	return &result
}

// Ping alimenta la sonda /readyz.
func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.pool.Ping(ctx)
}

func (s *Store) Close() { s.pool.Close() }

// CreateVerificationTokenParams contains an email-verification token hash.
type CreateVerificationTokenParams struct {
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
}

// RegistrationWriter groups the writes that must succeed atomically during
// account registration.
type RegistrationWriter interface {
	CreateUser(context.Context, CreateUserParams) (User, error)
	CreateVerificationToken(context.Context, CreateVerificationTokenParams) error
	InsertAuditEvent(context.Context, InsertAuditEventParams) (AuditEvent, error)
}

// EmailVerificationWriter groups the writes that must succeed atomically when
// a one-time email-verification token is consumed.
type EmailVerificationWriter interface {
	ConsumeEmailVerificationToken(context.Context, []byte) (User, error)
	InsertAuditEvent(context.Context, InsertAuditEventParams) (AuditEvent, error)
}

// WithinEmailVerificationTransaction consumes a verification token, activates
// its account, and records the audit event in a single PostgreSQL transaction.
func (s *Store) WithinEmailVerificationTransaction(ctx context.Context, fn func(EmailVerificationWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin email verification transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	writer := &emailVerificationWriter{queries: generated.New(tx)}
	if err := fn(writer); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit email verification transaction: %w", err)
	}
	return nil
}

// WithinRegistrationTransaction runs registration writes in one PostgreSQL
// transaction. A publisher-confirm failure returned by fn rolls it back.
func (s *Store) WithinRegistrationTransaction(ctx context.Context, fn func(RegistrationWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin registration transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	writer := &registrationWriter{queries: generated.New(tx)}
	if err := fn(writer); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit registration transaction: %w", err)
	}
	return nil
}

type registrationWriter struct{ queries *generated.Queries }

type emailVerificationWriter struct{ queries *generated.Queries }

func (w *emailVerificationWriter) ConsumeEmailVerificationToken(ctx context.Context, tokenHash []byte) (User, error) {
	user, err := w.queries.ConsumeEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		return User{}, fmt.Errorf("consume email verification token: %w", err)
	}
	return userFromGenerated(user), nil
}

func (w *emailVerificationWriter) InsertAuditEvent(ctx context.Context, params InsertAuditEventParams) (AuditEvent, error) {
	event, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(params.ActorUserID), Action: params.Action, ResourceType: nullableText(params.ResourceType), ResourceID: nullableText(params.ResourceID), Ip: copyAddr(params.IP), UserAgent: nullableText(params.UserAgent), Metadata: params.Metadata})
	if err != nil {
		return AuditEvent{}, fmt.Errorf("insert audit event: %w", err)
	}
	return auditEventFromGenerated(event), nil
}

func (w *registrationWriter) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	user, err := w.queries.CreateUser(ctx, generated.CreateUserParams{Email: params.Email, PasswordHash: params.PasswordHash, DisplayName: params.DisplayName})
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return userFromGenerated(user), nil
}

func (w *registrationWriter) CreateVerificationToken(ctx context.Context, params CreateVerificationTokenParams) error {
	if err := w.queries.CreateVerificationToken(ctx, generated.CreateVerificationTokenParams{UserID: params.UserID, TokenHash: params.TokenHash, ExpiresAt: pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true}}); err != nil {
		return fmt.Errorf("create verification token: %w", err)
	}
	return nil
}

func (w *registrationWriter) InsertAuditEvent(ctx context.Context, params InsertAuditEventParams) (AuditEvent, error) {
	event, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(params.ActorUserID), Action: params.Action, ResourceType: nullableText(params.ResourceType), ResourceID: nullableText(params.ResourceID), Ip: copyAddr(params.IP), UserAgent: nullableText(params.UserAgent), Metadata: params.Metadata})
	if err != nil {
		return AuditEvent{}, fmt.Errorf("insert audit event: %w", err)
	}
	return auditEventFromGenerated(event), nil
}
