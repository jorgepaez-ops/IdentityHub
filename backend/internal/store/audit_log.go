package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

func optionalAddr(addr *netip.Addr) *string {
	if addr == nil {
		return nil
	}
	value := addr.String()
	return &value
}

// ListAuditLog loads an immutable, keyset-paginated audit-log page through a
// parameterized sqlc query.
func (s *Store) ListAuditLog(ctx context.Context, input auditlog.ListInput) ([]auditlog.Event, error) {
	return s.listAuditLog(ctx, input, 101)
}

// ExportAuditLog runs the same filtered query as ListAuditLog with the export row cap (RF-019).
func (s *Store) ExportAuditLog(ctx context.Context, input auditlog.ListInput) ([]auditlog.Event, error) {
	return s.listAuditLog(ctx, input, auditlog.MaxExportRows+1)
}

func (s *Store) listAuditLog(ctx context.Context, input auditlog.ListInput, maxRows int) ([]auditlog.Event, error) {
	limit := input.Limit
	if limit < 1 {
		limit = 1
	}
	if limit > maxRows {
		limit = maxRows
	}
	params := generated.ListAuditLogParams{LimitCount: int32(limit)}
	if input.Action != nil {
		params.Action = pgtype.Text{String: *input.Action, Valid: true}
	}
	if input.ActorID != nil {
		params.ActorUserID = nullableUUID(input.ActorID)
	}
	if input.Since != nil {
		params.Since = pgtype.Timestamptz{Time: *input.Since, Valid: true}
	}
	if input.Cursor != nil {
		params.CursorID = pgtype.Int8{Int64: *input.Cursor, Valid: true}
	}
	rows, err := s.queries.ListAuditLog(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list audit log: %w", err)
	}
	result := make([]auditlog.Event, 0, len(rows))
	for _, row := range rows {
		metadata, err := auditlog.DecodeMetadata(row.Metadata)
		if err != nil {
			return nil, fmt.Errorf("decode audit event %d metadata: %w", row.ID, err)
		}
		result = append(result, auditlog.Event{
			ID: row.ID, ActorUserID: optionalUUID(row.ActorUserID), Action: row.Action,
			ResourceType: optionalText(row.ResourceType), ResourceID: optionalText(row.ResourceID),
			IP: optionalAddr(row.Ip), UserAgent: optionalText(row.UserAgent), Metadata: metadata,
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return result, nil
}

// RecordAuditExport appends the audit event of an export: who, in which format, with which filters and how many rows.
func (s *Store) RecordAuditExport(ctx context.Context, record auditlog.ExportRecord) error {
	filters := map[string]any{}
	if record.Action != nil {
		filters["action"] = *record.Action
	}
	if record.ActorID != nil {
		filters["actorId"] = record.ActorID.String()
	}
	if record.Since != nil {
		filters["since"] = record.Since.UTC().Format(time.RFC3339)
	}
	metadata, err := json.Marshal(map[string]any{"format": record.Format, "rows": record.Rows, "truncated": record.Truncated, "filters": filters})
	if err != nil {
		return fmt.Errorf("encode audit export metadata: %w", err)
	}
	resourceType := "audit_log"
	actor := record.RequestedBy
	if _, err := s.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{
		ActorUserID: nullableUUID(&actor), Action: auditlog.ExportAuditAction, ResourceType: nullableText(&resourceType),
		Ip: copyAddr(record.IP), UserAgent: nullableText(record.UserAgent), Metadata: metadata,
	}); err != nil {
		return fmt.Errorf("insert audit export event: %w", err)
	}
	return nil
}
