// Package events implementa el contrato de mensajería de
// specs/04-events/asyncapi.yaml.
//
// Los tipos se escriben a mano porque la generación de Go desde AsyncAPI no
// está madura (ver el encabezado del propio asyncapi.yaml). Lo que sí es
// automático es la verificación: TestEventContract_MatchesAsyncAPISchema valida
// cada evento serializado contra el JSON Schema del spec.
package events

import (
	"time"

	"github.com/google/uuid"
)

// Nombres de canal. Coinciden literalmente con las claves de `channels` del
// AsyncAPI y se usan como routing key en el exchange topic.
const (
	TypeUserRegistered         = "user.registered"
	TypeUserInvited            = "user.invited"
	TypeEmailVerified          = "user.email_verified"
	TypePasswordResetRequested = "user.password_reset_requested"
	TypePasswordResetCompleted = "user.password_reset_completed"
	TypeMfaChallengeIssued     = "security.mfa_challenge_issued"
	TypeRefreshReuseDetected   = "security.refresh_reuse_detected"
	TypeAccountLocked          = "security.account_locked"
)

// Topología declarada en specs/04-events/topology.md.
const (
	ExchangeEvents = "identity.events"
	ExchangeDLX    = "identity.dlx"
	QueueNotify    = "notifications"
	QueueDLQ       = "notifications.dlq"
)

// Envelope es el sobre común. `EventID` es lo que permite al worker ser
// idempotente: RabbitMQ garantiza entrega "al menos una vez", así que un
// reintento no debe traducirse en un segundo correo.
type Envelope struct {
	EventID    uuid.UUID `json:"eventId"`
	EventType  string    `json:"eventType"`
	OccurredAt time.Time `json:"occurredAt"`
	Version    int       `json:"version"`
	TraceID    string    `json:"traceId,omitempty"`
}

func NewEnvelope(eventType, traceID string) Envelope {
	return Envelope{
		EventID:    uuid.New(),
		EventType:  eventType,
		OccurredAt: time.Now().UTC(),
		Version:    1,
		TraceID:    traceID,
	}
}

type UserRegistered struct {
	Envelope
	Data struct {
		UserID            uuid.UUID `json:"userId"`
		Email             string    `json:"email"`
		DisplayName       string    `json:"displayName"`
		VerificationToken string    `json:"verificationToken"`
		ExpiresAt         time.Time `json:"expiresAt"`
	} `json:"data"`
}

// UserInvited is published when an administrator creates an employee account
// (T5, RF-001/RF-012, D6/D9). The worker renders InvitationToken into the
// account's set-password link; it never reaches logs (RNF-012).
type UserInvited struct {
	Envelope
	Data struct {
		UserID          uuid.UUID `json:"userId"`
		Email           string    `json:"email"`
		DisplayName     string    `json:"displayName"`
		InvitationToken string    `json:"invitationToken"`
		ExpiresAt       time.Time `json:"expiresAt"`
	} `json:"data"`
}

type EmailVerified struct {
	Envelope
	Data struct {
		UserID      uuid.UUID `json:"userId"`
		Email       string    `json:"email"`
		DisplayName string    `json:"displayName"`
	} `json:"data"`
}

type PasswordResetRequested struct {
	Envelope
	Data struct {
		Email         string    `json:"email"`
		ResetToken    string    `json:"resetToken"`
		ExpiresAt     time.Time `json:"expiresAt"`
		AccountExists bool      `json:"accountExists"`
	} `json:"data"`
}

// PasswordResetCompleted is always sent after a reset commits so the account
// holder learns that their password changed and sessions were closed (D14).
type PasswordResetCompleted struct {
	Envelope
	Data struct {
		UserID      uuid.UUID `json:"userId"`
		Email       string    `json:"email"`
		DisplayName string    `json:"displayName"`
		Unlocked    bool      `json:"unlocked"`
	} `json:"data"`
}

type MfaChallengeIssued struct {
	Envelope
	Data struct {
		UserID      uuid.UUID `json:"userId"`
		Email       string    `json:"email"`
		DisplayName string    `json:"displayName"`
		Code        string    `json:"code"`
		ExpiresAt   time.Time `json:"expiresAt"`
	} `json:"data"`
}

type RefreshReuseDetected struct {
	Envelope
	Data struct {
		UserID       uuid.UUID `json:"userId"`
		Email        string    `json:"email"`
		FamilyID     uuid.UUID `json:"familyId"`
		RevokedCount int       `json:"revokedCount"`
		IP           string    `json:"ip"`
		UserAgent    string    `json:"userAgent,omitempty"`
	} `json:"data"`
}

type AccountLocked struct {
	Envelope
	Data struct {
		UserID         uuid.UUID `json:"userId"`
		Email          string    `json:"email"`
		DisplayName    string    `json:"displayName"`
		LockedUntil    time.Time `json:"lockedUntil"`
		FailedAttempts int       `json:"failedAttempts"`
		IP             string    `json:"ip"`
	} `json:"data"`
}
