// Package audit records security events without exposing database internals to callers.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/api"
	"github.com/jorgepaez/identity-hub/internal/store"
)

// Action identifies a security event defined by RF-011 and the acceptance scenarios.
type Action string

const (
	UserRegistered       Action = "user_registered"
	EmailVerified        Action = "email_verified"
	LoginSucceeded       Action = "login_succeeded"
	LoginFailed          Action = "login_failed"
	Logout               Action = "logout"
	RefreshRotated       Action = "refresh_rotated"
	RefreshReuseDetected Action = "refresh_reuse_detected"
	PasswordChanged      Action = "password_changed"
	MFAEnabled           Action = "mfa_enabled"
	MFADisabled          Action = "mfa_disabled"
	RoleChanged          Action = "role_changed"
	AccountLocked        Action = "account_locked"
	UserDisabled         Action = "user_disabled"
)

// Event contains the application data for one immutable audit entry.
type Event struct {
	ActorUserID  *uuid.UUID
	Action       Action
	ResourceType *string
	ResourceID   *string
	Metadata     json.RawMessage
}

// Writer is the persistence boundary used by Record.
type Writer interface {
	InsertAuditEvent(context.Context, store.InsertAuditEventParams) (store.AuditEvent, error)
}

// Record appends a validated audit event with trusted request context.
func Record(ctx context.Context, writer Writer, request *http.Request, event Event) (store.AuditEvent, error) {
	if writer == nil {
		return store.AuditEvent{}, errors.New("audit writer is required")
	}
	if request == nil {
		return store.AuditEvent{}, errors.New("audit request is required")
	}
	if !event.Action.valid() {
		return store.AuditEvent{}, fmt.Errorf("invalid audit action %q", event.Action)
	}

	metadata, err := safeMetadata(event.Metadata)
	if err != nil {
		return store.AuditEvent{}, err
	}

	params := store.InsertAuditEventParams{
		ActorUserID:  event.ActorUserID,
		Action:       string(event.Action),
		ResourceType: event.ResourceType,
		ResourceID:   event.ResourceID,
		UserAgent:    optionalString(request.UserAgent()),
		Metadata:     metadata,
	}
	if ip, ok := api.ClientIPFrom(ctx); ok {
		params.IP = &ip
	}

	recorded, err := writer.InsertAuditEvent(ctx, params)
	if err != nil {
		return store.AuditEvent{}, fmt.Errorf("record audit event: %w", err)
	}
	return recorded, nil
}

func (a Action) valid() bool {
	switch a {
	case UserRegistered, EmailVerified, LoginSucceeded, LoginFailed, Logout, RefreshRotated,
		RefreshReuseDetected, PasswordChanged, MFAEnabled, MFADisabled, RoleChanged, AccountLocked, UserDisabled:
		return true
	default:
		return false
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func safeMetadata(metadata json.RawMessage) (json.RawMessage, error) {
	if len(metadata) == 0 {
		return json.RawMessage(`{}`), nil
	}

	var value any
	if err := json.Unmarshal(metadata, &value); err != nil {
		return nil, fmt.Errorf("audit metadata must be valid JSON: %w", err)
	}
	if containsSensitiveKey(value) {
		return nil, errors.New("audit metadata contains a sensitive field")
	}
	return append(json.RawMessage(nil), metadata...), nil
}

func containsSensitiveKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if sensitiveKey(key) || containsSensitiveKey(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if containsSensitiveKey(nested) {
				return true
			}
		}
	}
	return false
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	return strings.Contains(normalized, "password") || strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "code") || strings.Contains(normalized, "secret")
}
