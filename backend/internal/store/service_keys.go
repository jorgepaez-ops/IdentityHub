package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/servicekey"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// ServiceKeys returns the PostgreSQL repository used by the service key service (RF-018).
func (s *Store) ServiceKeys() servicekey.Repository { return &serviceKeyRepository{store: s} }

type serviceKeyRepository struct{ store *Store }

func (r *serviceKeyRepository) Create(ctx context.Context, params servicekey.CreateParams, event servicekey.AuditEvent) (servicekey.Key, error) {
	var key servicekey.Key
	err := r.store.inTransaction(ctx, "service key", func(queries *generated.Queries) error {
		row, err := queries.InsertServiceKey(ctx, generated.InsertServiceKeyParams{
			PublicID: params.PublicID, SecretHash: params.SecretHash, Name: params.Name, Scopes: params.Scopes,
			CreatedBy: nullableUUID(&params.CreatedBy), ExpiresAt: pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("insert service key: %w", err)
		}
		key = servicekey.Key{
			ID: row.ID, PublicID: row.PublicID, Name: row.Name, Scopes: row.Scopes, CreatedBy: optionalUUID(row.CreatedBy),
			CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time, LastUsedAt: optionalTime(row.LastUsedAt), RevokedAt: optionalTime(row.RevokedAt),
		}
		event.KeyID = &key.ID
		return insertServiceKeyAudit(ctx, queries, event)
	})
	return key, err
}

func (r *serviceKeyRepository) List(ctx context.Context) ([]servicekey.Key, error) {
	rows, err := r.store.queries.ListServiceKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list service keys: %w", err)
	}
	keys := make([]servicekey.Key, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, servicekey.Key{
			ID: row.ID, PublicID: row.PublicID, Name: row.Name, Scopes: row.Scopes, CreatedBy: optionalUUID(row.CreatedBy),
			CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time, LastUsedAt: optionalTime(row.LastUsedAt), RevokedAt: optionalTime(row.RevokedAt),
		})
	}
	return keys, nil
}

// Revoke locks the row, so two concurrent revocations write exactly one audit event.
func (r *serviceKeyRepository) Revoke(ctx context.Context, id uuid.UUID, event servicekey.AuditEvent) error {
	return r.store.inTransaction(ctx, "service key", func(queries *generated.Queries) error {
		row, err := queries.GetServiceKeyForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return servicekey.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock service key: %w", err)
		}
		if row.RevokedAt.Valid {
			return nil
		}
		if _, err := queries.RevokeServiceKey(ctx, id); err != nil {
			return fmt.Errorf("revoke service key: %w", err)
		}
		return insertServiceKeyAudit(ctx, queries, event)
	})
}

func (r *serviceKeyRepository) GetByPublicID(ctx context.Context, publicID string) (servicekey.Record, error) {
	row, err := r.store.queries.GetServiceKeyByPublicID(ctx, publicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return servicekey.Record{}, servicekey.ErrNotFound
	}
	if err != nil {
		return servicekey.Record{}, fmt.Errorf("get service key: %w", err)
	}
	return servicekey.Record{
		Key: servicekey.Key{
			ID: row.ID, PublicID: row.PublicID, Name: row.Name, Scopes: row.Scopes, CreatedBy: optionalUUID(row.CreatedBy),
			CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time, LastUsedAt: optionalTime(row.LastUsedAt), RevokedAt: optionalTime(row.RevokedAt),
		},
		SecretHash: row.SecretHash,
	}, nil
}

func (r *serviceKeyRepository) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	if err := r.store.queries.TouchServiceKeyLastUsed(ctx, generated.TouchServiceKeyLastUsedParams{ID: id, UsedAt: pgtype.Timestamptz{Time: at, Valid: true}}); err != nil {
		return fmt.Errorf("touch service key: %w", err)
	}
	return nil
}

func (r *serviceKeyRepository) RecordAudit(ctx context.Context, event servicekey.AuditEvent) error {
	return insertServiceKeyAudit(ctx, r.store.queries, event)
}

func insertServiceKeyAudit(ctx context.Context, queries *generated.Queries, event servicekey.AuditEvent) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal service key audit metadata: %w", err)
	}
	if event.Metadata == nil {
		metadata = []byte(`{}`)
	}
	resourceType := "service_key"
	params := generated.InsertAuditEventParams{ActorUserID: nullableUUID(event.ActorUserID), Action: event.Action, ResourceType: nullableText(&resourceType), Metadata: metadata}
	switch {
	case event.KeyID != nil:
		id := event.KeyID.String()
		params.ResourceID = nullableText(&id)
	case event.PublicID != "":
		params.ResourceID = nullableText(&event.PublicID)
	}
	if event.UserAgent != "" {
		params.UserAgent = nullableText(&event.UserAgent)
	}
	if event.IP.IsValid() {
		params.Ip = &event.IP
	}
	if _, err := queries.InsertAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("insert service key audit event: %w", err)
	}
	return nil
}

// inTransaction runs fn in one PostgreSQL transaction, committing only if it returns nil.
func (s *Store) inTransaction(ctx context.Context, name string, fn func(*generated.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s transaction: %w", name, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(generated.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s transaction: %w", name, err)
	}
	return nil
}
