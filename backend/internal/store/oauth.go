package store

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

type oauthRepository struct{ store *Store }

func (s *Store) OAuthRepository() oauthRepository { return oauthRepository{store: s} }

// ListOAuthClients loads every registered client from the applications table
// (ADR 0015). Callers validate the result with oauth.ValidateClients.
func (s *Store) ListOAuthClients(ctx context.Context) ([]oauth.Client, error) {
	rows, err := s.queries.ListOAuthApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("list oauth applications: %w", err)
	}
	clients := make([]oauth.Client, 0, len(rows))
	for _, row := range rows {
		clients = append(clients, oauth.Client{ID: row.ClientID, RedirectURI: row.RedirectUri, Origin: row.AllowedOrigin, OrganizationID: row.OrganizationID})
	}
	return clients, nil
}

func (s *Store) PurgeAuthorizationCodes(ctx context.Context) (int64, error) {
	count, err := s.queries.PurgeAuthorizationCodes(ctx)
	if err != nil {
		return 0, fmt.Errorf("purge authorization codes: %w", err)
	}
	return count, nil
}

func (s *Store) PurgeHubSessions(ctx context.Context) (int64, error) {
	count, err := s.queries.PurgeHubSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("purge hub sessions: %w", err)
	}
	return count, nil
}

// WithinAuthorizationCodeTransaction keeps code consumption, role lookup and
// its audit record indivisible: an infrastructure failure after the row lock
// must leave the code available for a later valid exchange.
func (r oauthRepository) WithinAuthorizationCodeTransaction(ctx context.Context, fn func(oauth.ExchangeWriter) error) error {
	tx, err := r.store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin authorization code exchange: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&oauthWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit authorization code exchange: %w", err)
	}
	return nil
}

type oauthWriter struct{ queries *generated.Queries }

func (r oauthRepository) CreateAuthorizationCode(ctx context.Context, code oauth.CreateCode) error {
	rows, err := r.store.queries.CreateAuthorizationCode(ctx, generated.CreateAuthorizationCodeParams{ID: code.ID, UserID: code.UserID, CodeHash: code.CodeHash, RedirectUri: code.RedirectURI, CodeChallenge: code.CodeChallenge, ExpiresAt: pgtype.Timestamptz{Time: code.ExpiresAt, Valid: true}, ClientID: code.ClientID})
	if err != nil {
		return fmt.Errorf("create authorization code: %w", err)
	}
	if rows != 1 {
		return oauth.ErrInvalidClient
	}
	return nil
}
func (w *oauthWriter) ExchangeAuthorizationCode(ctx context.Context, hash []byte, clientID, redirectURI, challenge string, usedAt time.Time) (result oauth.StoredCode, err error) {
	row, err := w.queries.GetAuthorizationCodeForUpdate(ctx, generated.GetAuthorizationCodeForUpdateParams{CodeHash: hash, ExpiresAt: pgtype.Timestamptz{Time: usedAt, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		owner, ownerErr := w.queries.GetUsedAuthorizationCodeOwner(ctx, hash)
		if ownerErr == nil {
			return oauth.StoredCode{UserID: owner}, oauth.ErrAuthorizationCodeReused
		}
		if !errors.Is(ownerErr, pgx.ErrNoRows) {
			return result, fmt.Errorf("look up used authorization code: %w", ownerErr)
		}
		return result, oauth.ErrAuthorizationCodeInvalid
	}
	if err != nil {
		return result, fmt.Errorf("lock authorization code: %w", err)
	}
	if row.ClientID != clientID || row.RedirectUri != redirectURI || subtle.ConstantTimeCompare([]byte(challenge), []byte(row.CodeChallenge)) != 1 {
		return result, oauth.ErrAuthorizationCodeInvalid
	}
	rows, err := w.queries.MarkAuthorizationCodeUsed(ctx, generated.MarkAuthorizationCodeUsedParams{ID: row.ID, UsedAt: pgtype.Timestamptz{Time: usedAt, Valid: true}})
	if err != nil {
		return result, fmt.Errorf("consume authorization code: %w", err)
	}
	if rows != 1 {
		return result, oauth.ErrAuthorizationCodeInvalid
	}
	return oauth.StoredCode{ID: row.ID, UserID: row.UserID, ClientID: row.ClientID, RedirectURI: row.RedirectUri, CodeChallenge: row.CodeChallenge, ExpiresAt: row.ExpiresAt.Time, UsedAt: usedAt}, nil
}
func (r oauthRepository) GetHubSessionUser(ctx context.Context, hash []byte) (uuid.UUID, error) {
	userID, err := r.store.queries.GetHubSessionUser(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, oauth.ErrHubSessionInvalid
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get hub session: %w", err)
	}
	return userID, nil
}
func (w *oauthWriter) ListApplicationRolesForUser(ctx context.Context, userID uuid.UUID, clientID string) ([]string, error) {
	roles, err := w.queries.ListApplicationRolesForUser(ctx, generated.ListApplicationRolesForUserParams{UserID: userID, ClientID: clientID})
	if err != nil {
		return nil, fmt.Errorf("list oauth application roles: %w", err)
	}
	return roles, nil
}

// GetAuthorizationAccess gathers what /oauth/authorize needs to accept or refuse a user.
func (r oauthRepository) GetAuthorizationAccess(ctx context.Context, userID uuid.UUID, clientID string) (oauth.AuthorizationAccess, error) {
	row, err := r.store.queries.GetOAuthAuthorizationAccess(ctx, generated.GetOAuthAuthorizationAccessParams{ID: userID, ClientID: clientID})
	if err != nil {
		return oauth.AuthorizationAccess{}, fmt.Errorf("get oauth authorization access: %w", err)
	}
	return oauth.AuthorizationAccess{UserOrganizationID: row.UserOrganizationID, ApplicationOrganizationID: row.ApplicationOrganizationID, HasApplicationRole: row.HasApplicationRole}, nil
}
func (w *oauthWriter) ListPermissionKeysForRolesAndApplication(ctx context.Context, roleNames []string, clientID string) ([]string, error) {
	keys, err := w.queries.ListPermissionKeysForRolesAndApplication(ctx, generated.ListPermissionKeysForRolesAndApplicationParams{RoleNames: roleNames, ClientID: clientID})
	if err != nil {
		return nil, fmt.Errorf("list oauth application permissions: %w", err)
	}
	return keys, nil
}
func (r oauthRepository) InsertAuditEvent(ctx context.Context, event oauth.AuditEvent) error {
	return insertOAuthAuditEvent(ctx, r.store.queries, event)
}

func (w *oauthWriter) InsertAuditEvent(ctx context.Context, event oauth.AuditEvent) error {
	return insertOAuthAuditEvent(ctx, w.queries, event)
}

func insertOAuthAuditEvent(ctx context.Context, queries *generated.Queries, event oauth.AuditEvent) error {
	fields := event.Metadata
	if fields == nil {
		fields = map[string]string{}
	}
	metadata, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal oauth audit metadata: %w", err)
	}
	var actor *uuid.UUID
	if event.ActorUserID != uuid.Nil {
		actor = &event.ActorUserID
	}
	_, err = queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(actor), Action: event.Action, Metadata: metadata})
	if err != nil {
		return fmt.Errorf("insert oauth audit event: %w", err)
	}
	return nil
}
