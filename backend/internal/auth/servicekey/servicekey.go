// Package servicekey implements service keys for machine-to-machine access
// (RF-018, ADR 0014). A key looks like ihk_<publicId>_<secret>: the public id
// locates the row, only the SHA-256 of the 256-bit secret is stored, and every
// authentication failure collapses into one error so callers learn nothing
// about why a key was refused.
package servicekey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Scopes form a closed list; an operation that accepts service keys names the one it needs.
const ScopeAuditLogRead = "audit-log:read"

// Audit actions written by this package. internal/audit aliases them (it cannot be
// imported from here without an import cycle through internal/api).
const (
	ActionCreated  = "service_key_created"
	ActionRevoked  = "service_key_revoked"
	ActionRejected = "service_key_rejected"
)

const (
	// KeyPrefix marks a bearer value as a service key and not as an access token.
	KeyPrefix = "ihk_"

	publicIDBytes = 8
	secretBytes   = 32
	// MaxNameLength bounds the label of a key, in characters.
	MaxNameLength = 100
	// MaxExpiresInDays is the longest allowed lifetime; keys never live forever.
	MaxExpiresInDays = 365
)

var (
	ErrInvalidInput      = errors.New("invalid service key request")
	ErrNotFound          = errors.New("service key not found")
	ErrUnauthenticated   = errors.New("invalid service key")
	ErrInsufficientScope = errors.New("service key lacks the required scope")
	errUnavailable       = errors.New("service key service is unavailable")
)

// InputError names the field that failed validation; it matches ErrInvalidInput.
type InputError struct{ Field, Detail string }

func (e *InputError) Error() string { return e.Field + ": " + e.Detail }
func (e *InputError) Is(target error) bool {
	return target == ErrInvalidInput
}

// Key is the metadata of a service key. It never carries the secret or its hash.
type Key struct {
	ID         uuid.UUID
	PublicID   string
	Name       string
	Scopes     []string
	CreatedAt  time.Time
	CreatedBy  *uuid.UUID
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// Prefix is the public part of the key, safe to show and to log.
func (k Key) Prefix() string { return KeyPrefix + k.PublicID }

// Record is a stored key with the hash used to verify it; it never leaves this package's callers' repositories.
type Record struct {
	Key
	SecretHash []byte
}

// Created is a freshly issued key. FullKey holds the complete ihk_<publicId>_<secret> value and
// exists only in the issuing response.
type Created struct {
	Key
	FullKey string
}

// AuditEvent is one audit entry written with a key operation. Metadata never holds the secret.
type AuditEvent struct {
	ActorUserID *uuid.UUID
	Action      string
	KeyID       *uuid.UUID
	PublicID    string
	Metadata    map[string]any
	IP          netip.Addr
	UserAgent   string
}

type CreateParams struct {
	PublicID   string
	SecretHash []byte
	Name       string
	Scopes     []string
	CreatedBy  uuid.UUID
	ExpiresAt  time.Time
}

// Repository is the persistence boundary. Create and Revoke write their audit event in the same
// transaction as the change; Revoke writes it only when the key was not already revoked.
type Repository interface {
	Create(context.Context, CreateParams, AuditEvent) (Key, error)
	List(context.Context) ([]Key, error)
	Revoke(context.Context, uuid.UUID, AuditEvent) error
	GetByPublicID(context.Context, string) (Record, error)
	TouchLastUsed(context.Context, uuid.UUID, time.Time) error
	RecordAudit(context.Context, AuditEvent) error
}

type CreateInput struct {
	ActorUserID   uuid.UUID
	Name          string
	Scopes        []string
	ExpiresInDays int
	IP            netip.Addr
	UserAgent     string
}

type RevokeInput struct {
	ActorUserID uuid.UUID
	IP          netip.Addr
	UserAgent   string
}

// RequestMeta is the request context recorded when a key is rejected.
type RequestMeta struct {
	IP        netip.Addr
	UserAgent string
}

// Principal identifies the service key that authenticated a request.
type Principal struct {
	KeyID    uuid.UUID
	PublicID string
	Scopes   []string
}

type Manager interface {
	Create(context.Context, CreateInput) (Created, error)
	List(context.Context) ([]Key, error)
	Revoke(context.Context, uuid.UUID, RevokeInput) error
	Authenticate(ctx context.Context, rawKey, requiredScope string, meta RequestMeta) (Principal, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func New(repository Repository) *Service { return &Service{repository: repository, now: time.Now} }

// decoyHash makes an unknown public id cost the same comparison as a known one.
var decoyHash = sha256.Sum256([]byte("service-key-decoy"))

func (s *Service) Create(ctx context.Context, input CreateInput) (Created, error) {
	if s.repository == nil {
		return Created{}, errUnavailable
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return Created{}, &InputError{Field: "name", Detail: fmt.Sprintf("must be 1 to %d characters", MaxNameLength)}
	}
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil {
		return Created{}, err
	}
	if input.ExpiresInDays < 1 || input.ExpiresInDays > MaxExpiresInDays {
		return Created{}, &InputError{Field: "expiresInDays", Detail: fmt.Sprintf("must be between 1 and %d", MaxExpiresInDays)}
	}
	publicID, secret, err := generate()
	if err != nil {
		return Created{}, err
	}
	hash := sha256.Sum256([]byte(secret))
	expiresAt := s.now().Add(time.Duration(input.ExpiresInDays) * 24 * time.Hour)
	actor := input.ActorUserID
	event := AuditEvent{
		ActorUserID: &actor, Action: ActionCreated, PublicID: publicID, IP: input.IP, UserAgent: input.UserAgent,
		Metadata: map[string]any{"publicId": publicID, "name": name, "scopes": scopes, "expiresAt": expiresAt.UTC().Format(time.RFC3339)},
	}
	key, err := s.repository.Create(ctx, CreateParams{PublicID: publicID, SecretHash: hash[:], Name: name, Scopes: scopes, CreatedBy: actor, ExpiresAt: expiresAt}, event)
	if err != nil {
		return Created{}, err
	}
	return Created{Key: key, FullKey: KeyPrefix + publicID + "_" + secret}, nil
}

func (s *Service) List(ctx context.Context) ([]Key, error) {
	if s.repository == nil {
		return nil, errUnavailable
	}
	return s.repository.List(ctx)
}

// Revoke is idempotent: revoking a revoked key succeeds and writes no second audit event.
func (s *Service) Revoke(ctx context.Context, id uuid.UUID, input RevokeInput) error {
	if s.repository == nil {
		return errUnavailable
	}
	actor := input.ActorUserID
	return s.repository.Revoke(ctx, id, AuditEvent{ActorUserID: &actor, Action: ActionRevoked, KeyID: &id, IP: input.IP, UserAgent: input.UserAgent})
}

// Authenticate verifies rawKey for requiredScope. Every cause of failure (malformed, unknown id,
// wrong secret, expired, revoked) returns the same ErrUnauthenticated and is audited with the
// real reason; a valid key without the scope returns ErrInsufficientScope.
func (s *Service) Authenticate(ctx context.Context, rawKey, requiredScope string, meta RequestMeta) (Principal, error) {
	if s.repository == nil {
		return Principal{}, errUnavailable
	}
	publicID, secret, wellFormed := parse(rawKey)
	reject := func(reason string, keyID *uuid.UUID) (Principal, error) {
		s.auditRejection(ctx, publicID, wellFormed, reason, requiredScope, keyID, meta)
		return Principal{}, ErrUnauthenticated
	}
	if !wellFormed {
		return reject("malformed", nil)
	}
	record, err := s.repository.GetByPublicID(ctx, publicID)
	notFound := errors.Is(err, ErrNotFound)
	if err != nil && !notFound {
		return Principal{}, fmt.Errorf("load service key: %w", err)
	}
	stored := record.SecretHash
	if notFound || len(stored) != sha256.Size {
		stored = decoyHash[:]
	}
	presented := sha256.Sum256([]byte(secret))
	match := subtle.ConstantTimeCompare(presented[:], stored) == 1
	switch {
	case notFound:
		return reject("unknown_key", nil)
	case !match:
		return reject("bad_secret", &record.ID)
	case record.RevokedAt != nil:
		return reject("revoked", &record.ID)
	case !s.now().Before(record.ExpiresAt):
		return reject("expired", &record.ID)
	}
	if !hasScope(record.Scopes, requiredScope) {
		s.auditRejection(ctx, publicID, true, "insufficient_scope", requiredScope, &record.ID, meta)
		return Principal{}, ErrInsufficientScope
	}
	// A failed last_used_at write must not fail an otherwise valid request.
	_ = s.repository.TouchLastUsed(ctx, record.ID, s.now())
	return Principal{KeyID: record.ID, PublicID: record.PublicID, Scopes: append([]string(nil), record.Scopes...)}, nil
}

func (s *Service) auditRejection(ctx context.Context, publicID string, wellFormed bool, reason, scope string, keyID *uuid.UUID, meta RequestMeta) {
	event := AuditEvent{Action: ActionRejected, KeyID: keyID, IP: meta.IP, UserAgent: meta.UserAgent, Metadata: map[string]any{"reason": reason, "scope": scope}}
	// Only a well-formed public id (16 hex characters) is recorded: anything else is attacker-controlled text.
	if wellFormed {
		event.PublicID = publicID
		event.Metadata["publicId"] = publicID
	}
	_ = s.repository.RecordAudit(ctx, event)
}

func hasScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func normalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, &InputError{Field: "scopes", Detail: "must include at least one scope"}
	}
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope != ScopeAuditLogRead {
			return nil, &InputError{Field: "scopes", Detail: "contains an unknown scope"}
		}
		if !hasScope(result, scope) {
			result = append(result, scope)
		}
	}
	return result, nil
}

func generate() (publicID, secret string, err error) {
	id := make([]byte, publicIDBytes)
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(id); err != nil {
		return "", "", fmt.Errorf("generate service key id: %w", err)
	}
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate service key secret: %w", err)
	}
	return hex.EncodeToString(id), base64.RawURLEncoding.EncodeToString(raw), nil
}

// parse splits ihk_<publicId>_<secret>. The public id is hex, so the first underscore after the
// prefix separates it from the base64url secret (which may itself contain underscores).
func parse(raw string) (publicID, secret string, ok bool) {
	rest, found := strings.CutPrefix(raw, KeyPrefix)
	if !found || len(rest) > 2*publicIDBytes+1+64 {
		return "", "", false
	}
	publicID, secret, found = strings.Cut(rest, "_")
	if !found || len(publicID) != 2*publicIDBytes || len(secret) != base64.RawURLEncoding.EncodedLen(secretBytes) {
		return "", "", false
	}
	if _, err := hex.DecodeString(publicID); err != nil || publicID != strings.ToLower(publicID) {
		return "", "", false
	}
	return publicID, secret, true
}
