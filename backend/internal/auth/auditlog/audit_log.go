// Package auditlog implements read-only access to the immutable audit trail.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event is one audit record returned to an administrator.
type Event struct {
	ID           int64
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType *string
	ResourceID   *string
	IP           *string
	UserAgent    *string
	Metadata     map[string]any
	CreatedAt    time.Time
}

// ListInput contains the optional filters and keyset cursor for the audit trail.
type ListInput struct {
	Action  *string
	ActorID *uuid.UUID
	Since   *time.Time
	Cursor  *int64
	Limit   int
}

// Repository is the persistence boundary for audit-log queries.
type Repository interface {
	ListAuditLog(context.Context, ListInput) ([]Event, error)
}

// Reader provides read-only audit-log access to the HTTP API.
type Reader interface {
	List(context.Context, ListInput) ([]Event, error)
}

// Service applies page bounds before querying the audit-log repository.
type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, input ListInput) ([]Event, error) {
	if s.repository == nil {
		return nil, errors.New("audit log service is unavailable")
	}
	if input.Limit < 1 {
		input.Limit = 1
	}
	// Handlers request one extra event to determine whether the public page has
	// a successor. The public limit remains capped at 100.
	if input.Limit > 101 {
		input.Limit = 101
	}
	events, err := s.repository.ListAuditLog(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("list audit log: %w", err)
	}
	return events, nil
}

// DecodeMetadata converts the jsonb value returned by PostgreSQL into the
// object required by the public API contract.
func DecodeMetadata(raw json.RawMessage) (map[string]any, error) {
	metadata := make(map[string]any)
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("decode audit metadata: %w", err)
	}
	return metadata, nil
}
