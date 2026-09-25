package store

import (
	"context"
	"fmt"
	"net/netip"

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
	limit := input.Limit
	if limit < 1 {
		limit = 1
	}
	if limit > 101 {
		limit = 101
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
