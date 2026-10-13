// Package auditlog implements read-only access to the immutable audit trail.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
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

// MaxExportRows bounds one export (RF-019). Larger filters are truncated and flagged.
const MaxExportRows = 10000

// ExportAuditAction is the audit action recorded for every export.
const ExportAuditAction = "audit_log_exported"

// ExportInput carries the list filters plus the requester context that the export audit event needs.
type ExportInput struct {
	Action      *string
	ActorID     *uuid.UUID
	Since       *time.Time
	Format      string
	RequestedBy uuid.UUID
	IP          *netip.Addr
	UserAgent   *string
}

// ExportResult is the bounded set of events to write out.
type ExportResult struct {
	Events    []Event
	Truncated bool
}

// ExportRecord is what gets persisted as the audit event of an export.
type ExportRecord struct {
	ExportInput
	Rows      int
	Truncated bool
}

// Repository is the persistence boundary for audit-log queries.
type Repository interface {
	ListAuditLog(context.Context, ListInput) ([]Event, error)
	// ExportAuditLog returns up to input.Limit events (at most MaxExportRows+1) newest first.
	ExportAuditLog(context.Context, ListInput) ([]Event, error)
	RecordAuditExport(context.Context, ExportRecord) error
}

// Reader provides audit-log access to the HTTP API.
type Reader interface {
	List(context.Context, ListInput) ([]Event, error)
	Export(context.Context, ExportInput) (ExportResult, error)
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

// Export loads the filtered events (bounded to MaxExportRows) and records the export in the audit
// log. It fails closed: when the audit event cannot be written, no data is returned.
func (s *Service) Export(ctx context.Context, input ExportInput) (ExportResult, error) {
	if s.repository == nil {
		return ExportResult{}, errors.New("audit log service is unavailable")
	}
	// One extra row tells whether the filter matched more than the cap.
	events, err := s.repository.ExportAuditLog(ctx, ListInput{Action: input.Action, ActorID: input.ActorID, Since: input.Since, Limit: MaxExportRows + 1})
	if err != nil {
		return ExportResult{}, fmt.Errorf("export audit log: %w", err)
	}
	result := ExportResult{Events: events}
	if len(events) > MaxExportRows {
		result.Events, result.Truncated = events[:MaxExportRows], true
	}
	record := ExportRecord{ExportInput: input, Rows: len(result.Events), Truncated: result.Truncated}
	if err := s.repository.RecordAuditExport(ctx, record); err != nil {
		return ExportResult{}, fmt.Errorf("record audit log export: %w", err)
	}
	return result, nil
}
