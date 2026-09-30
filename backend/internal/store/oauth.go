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
func (r oauthRepository) ExchangeAuthorizationCode(ctx context.Context, hash []byte, clientID, redirectURI, challenge string, usedAt time.Time) (result oauth.StoredCode, err error) {
	tx, err := r.store.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("begin authorization code exchange: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := generated.New(tx)
	row, err := queries.GetAuthorizationCodeForUpdate(ctx, generated.GetAuthorizationCodeForUpdateParams{CodeHash: hash, ExpiresAt: pgtype.Timestamptz{Time: usedAt, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		owner, ownerErr := queries.GetUsedAuthorizationCodeOwner(ctx, hash)
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
	rows, err := queries.MarkAuthorizationCodeUsed(ctx, generated.MarkAuthorizationCodeUsedParams{ID: row.ID, UsedAt: pgtype.Timestamptz{Time: usedAt, Valid: true}})
	if err != nil {
		return result, fmt.Errorf("consume authorization code: %w", err)
	}
	if rows != 1 {
		return result, oauth.ErrAuthorizationCodeInvalid
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit authorization code exchange: %w", err)
	}
	return oauth.StoredCode{ID: row.ID, UserID: row.UserID, ClientID: row.ClientID, RedirectURI: row.RedirectUri, CodeChallenge: row.CodeChallenge, ExpiresAt: row.ExpiresAt.Time, UsedAt: usedAt}, nil
}
func (r oauthRepository) GetHubSessionUser(ctx context.Context, hash []byte) (uuid.UUID, error) {
	userID, err := r.store.queries.GetHubSessionUser(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, oauth.ErrAuthorizationCodeInvalid
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get hub session: %w", err)
	}
	return userID, nil
}
func (r oauthRepository) ListRolesForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	roles, err := r.store.queries.ListRolesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list oauth user roles: %w", err)
	}
	return roles, nil
}
func (r oauthRepository) InsertAuditEvent(ctx context.Context, event oauth.AuditEvent) error {
	metadata, err := json.Marshal(map[string]string{})
	if err != nil {
		return fmt.Errorf("marshal oauth audit metadata: %w", err)
	}
	var actor *uuid.UUID
	if event.ActorUserID != uuid.Nil {
		actor = &event.ActorUserID
	}
	_, err = r.store.InsertAuditEvent(ctx, InsertAuditEventParams{ActorUserID: actor, Action: event.Action, Metadata: metadata})
	if err != nil {
		return fmt.Errorf("insert oauth audit event: %w", err)
	}
	return nil
}
