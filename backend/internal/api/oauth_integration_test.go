//go:build integration

package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/auth/logout"
	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/passwordreset"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type hubFixture struct {
	pool       *pgxpool.Pool
	repository *store.Store
	server     *Server
	client     oauth.Client
	userID     uuid.UUID
	hubCookie  string
	refreshRaw []byte
}

func newHubFixture(t *testing.T, email string) hubFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: email, PasswordHash: "$argon2id$placeholder", DisplayName: "Hub"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	hubRaw, refreshRaw := []byte("hub-session-integration-raw-bytes"), []byte("refresh-integration-raw-bytes")
	hubHash, refreshHash := sha256.Sum256(hubRaw), sha256.Sum256(refreshRaw)
	if _, err := pool.Exec(ctx, `INSERT INTO hub_sessions (user_id, token_hash, expires_at) VALUES ($1,$2,$3)`, user.ID, hubHash[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at) VALUES ($1,$2,$3,$4)`, user.ID, refreshHash[:], uuid.New(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}
	server := NewServer(nil, "test", nil)
	server.SetOAuthService(oauth.New(repository.OAuthRepository(), client, nil, time.Now), repository.OAuthRepository(), client, "http://identityhub.localhost:8080/login", time.Hour)
	server.SetLogoutService(logout.New(repository))
	return hubFixture{pool: pool, repository: repository, server: server, client: client, userID: user.ID, hubCookie: base64.RawURLEncoding.EncodeToString(hubRaw), refreshRaw: refreshRaw}
}

// authorize replays GET /oauth/authorize with the hub_session cookie and
// reports whether the Hub issued a code (redirect to the client) or sent the
// browser to the login page.
func (f hubFixture) authorize(t *testing.T) (issued bool) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, oauthAuthorizePath(f.client), nil)
	request.AddCookie(&http.Cookie{Name: hubSessionCookieName, Value: f.hubCookie})
	response := httptest.NewRecorder()
	f.server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("authorize status=%d body=%s", response.Code, response.Body.String())
	}
	location := response.Header().Get("Location")
	switch {
	case strings.HasPrefix(location, f.client.RedirectURI+"?code="):
		return true
	case strings.HasPrefix(location, "http://identityhub.localhost:8080/login?"):
		return false
	}
	t.Fatalf("unexpected redirect %q", location)
	return false
}

func TestRF007_TrasLogoutLaSesionHubYaNoEmiteCodigos(t *testing.T) {
	fixture := newHubFixture(t, "hub-logout@example.test")
	if !fixture.authorize(t) {
		t.Fatal("active hub session must issue a code before logout")
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: base64.RawURLEncoding.EncodeToString(fixture.refreshRaw)})
	response := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d body=%s", response.Code, response.Body.String())
	}
	if fixture.authorize(t) {
		t.Fatal("hub session still issues codes after logout")
	}
}

func TestRF015_TrasRestablecerContrasenaLaSesionHubYaNoEmiteCodigos(t *testing.T) {
	fixture := newHubFixture(t, "hub-reset@example.test")
	if !fixture.authorize(t) {
		t.Fatal("active hub session must issue a code before the reset")
	}
	ctx := context.Background()
	raw := []byte("password-reset-hub-integration-token")
	hash := sha256.Sum256(raw)
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1,$2,'password_reset',$3)`, fixture.userID, hash[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	service := passwordreset.New(fixture.repository, resetPublisher{}, resetHasher{}, bytes.NewReader(make([]byte, 32)), time.Now)
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	if fixture.authorize(t) {
		t.Fatal("hub session still issues codes after password reset")
	}
}

type resetPublisher struct{}

func (resetPublisher) Publish(context.Context, string, any) error { return nil }

type resetHasher struct{}

func (resetHasher) Hash(value string) (string, error) { return password.Hash(value) }
