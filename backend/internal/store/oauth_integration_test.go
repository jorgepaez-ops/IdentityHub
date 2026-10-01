//go:build integration

package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF020_PostgresCodigoPKCEAtomicoYHashOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "oauth@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "OAuth"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'contabilidad.senior'`, user.ID); err != nil {
		t.Fatal(err)
	}
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback"}
	service := oauth.New(repository.OAuthRepository(), client, bytes.NewReader(append(bytes.Repeat([]byte{7}, 32), bytes.Repeat([]byte{8}, 96)...)), time.Now)
	issued, err := service.Authorize(ctx, user.ID, oauth.AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, ResponseType: "code", State: "state", CodeChallenge: challengeForIntegration("verifier"), CodeChallengeMethod: "S256"})
	if err != nil {
		t.Fatal(err)
	}
	var persisted []byte
	if err := pool.QueryRow(ctx, `SELECT code_hash FROM authorization_codes WHERE user_id = $1`, user.ID).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	rawCode, err := base64.RawURLEncoding.DecodeString(issued.Code)
	if err != nil {
		t.Fatal(err)
	}
	expectedCodeHash := sha256.Sum256(rawCode)
	if !bytes.Equal(persisted, expectedCodeHash[:]) || bytes.Equal(persisted, rawCode) {
		t.Fatal("authorization code was persisted in plaintext")
	}
	publisher := &mfaCodePublisher{}
	signer, err := token.New(make([]byte, 32), "issuer", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mfaService := mfa.New(repository, publisher, bytes.NewReader(bytes.Repeat([]byte{9}, 256)), time.Now).WithTokenService(signer, time.Hour).WithHubSessionTTL(time.Hour)
	mfaChallenge, err := mfaService.Issue(ctx, mfa.User{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	mfaResult, err := mfaService.Verify(ctx, mfa.VerifyInput{Token: mfaChallenge.Token, Code: publisher.code})
	if err != nil {
		t.Fatal(err)
	}
	hubRaw, err := base64.RawURLEncoding.DecodeString(mfaResult.HubSessionToken)
	if err != nil {
		t.Fatal(err)
	}
	hubHash := sha256.Sum256(hubRaw)
	var storedHub []byte
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM hub_sessions WHERE user_id = $1`, user.ID).Scan(&storedHub); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(storedHub, hubRaw) {
		t.Fatal("hub session was persisted in plaintext")
	}
	var reader oauth.HubSessionReader = repository.OAuthRepository()
	if got, err := reader.GetHubSessionUser(ctx, hubHash[:]); err != nil || got != user.ID {
		t.Fatalf("hub session user=%s err=%v", got, err)
	}
	wrong := oauth.ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "wrong"}
	if _, err := service.Exchange(ctx, wrong); !errors.Is(err, oauth.ErrAuthorizationCodeInvalid) {
		t.Fatalf("wrong verifier=%v", err)
	}
	if _, err := service.Exchange(ctx, oauth.ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}); err != nil {
		t.Fatalf("correct verifier after wrong one: %v", err)
	}
	issued, err = service.Authorize(ctx, user.ID, oauth.AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, ResponseType: "code", State: "state", CodeChallenge: challengeForIntegration("verifier"), CodeChallengeMethod: "S256"})
	if err != nil {
		t.Fatal(err)
	}
	var successes int
	var lock sync.Mutex
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := service.Exchange(ctx, oauth.ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}); err == nil {
				lock.Lock()
				successes++
				lock.Unlock()
			}
		}()
	}
	group.Wait()
	if successes != 1 {
		t.Fatalf("concurrent exchanges succeeded=%d, want 1", successes)
	}
}

func TestRF020_PostgresReusoDeCodigoAuditaConElPropietarioComoActor(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "oauth-reuse@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "Reuse"})
	if err != nil {
		t.Fatal(err)
	}
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback"}
	service := oauth.New(repository.OAuthRepository(), client, bytes.NewReader(bytes.Repeat([]byte{5}, 128)), time.Now)
	issued, err := service.Authorize(ctx, user.ID, oauth.AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, ResponseType: "code", State: "state", CodeChallenge: challengeForIntegration("verifier"), CodeChallengeMethod: "S256"})
	if err != nil {
		t.Fatal(err)
	}
	input := oauth.ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}
	if _, err := service.Exchange(ctx, input); err != nil {
		t.Fatal(err)
	}
	countReused := func(actorSet bool) int {
		var count int
		query := `SELECT count(*) FROM audit_log WHERE action = 'authorization_code_reused' AND actor_user_id IS NULL`
		if actorSet {
			query = `SELECT count(*) FROM audit_log WHERE action = 'authorization_code_reused' AND actor_user_id = $1`
		}
		args := []any{}
		if actorSet {
			args = append(args, user.ID)
		}
		if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	if _, err := service.Exchange(ctx, input); !errors.Is(err, oauth.ErrAuthorizationCodeInvalid) {
		t.Fatalf("second exchange=%v, want invalid grant", err)
	}
	if got := countReused(true); got != 1 {
		t.Fatalf("reuse audits with owner=%d, want 1", got)
	}
	garbage := oauth.ExchangeInput{Code: base64.RawURLEncoding.EncodeToString([]byte("never-issued")), ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"}
	if _, err := service.Exchange(ctx, garbage); !errors.Is(err, oauth.ErrAuthorizationCodeInvalid) {
		t.Fatalf("unknown code=%v", err)
	}
	if got := countReused(false); got != 0 {
		t.Fatalf("unauthenticated garbage wrote %d audit rows", got)
	}
}

func TestRF020_PostgresPurgaCodigosYSesionesHubInutilizables(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "oauth-purge@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "Purge"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO authorization_codes (user_id, application_id, code_hash, redirect_uri, code_challenge, expires_at, used_at) SELECT $1, id, $2, redirect_uri, 'challenge', $3, $4 FROM applications WHERE client_id = 'contabilidad'`, user.ID, bytes.Repeat([]byte{1}, 32), time.Now().Add(-time.Hour), time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO hub_sessions (user_id, token_hash, expires_at, revoked_at) VALUES ($1,$2,$3,$4)`, user.ID, bytes.Repeat([]byte{2}, 32), time.Now().Add(-time.Hour), time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if count, err := repository.PurgeAuthorizationCodes(ctx); err != nil || count != 1 {
		t.Fatalf("purged authorization codes=%d err=%v", count, err)
	}
	if count, err := repository.PurgeHubSessions(ctx); err != nil || count != 1 {
		t.Fatalf("purged hub sessions=%d err=%v", count, err)
	}
}

func challengeForIntegration(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type mfaCodePublisher struct{ code string }

func (p *mfaCodePublisher) Publish(_ context.Context, _ string, event any) error {
	if issued, ok := event.(events.MfaChallengeIssued); ok {
		p.code = issued.Data.Code
	}
	return nil
}
