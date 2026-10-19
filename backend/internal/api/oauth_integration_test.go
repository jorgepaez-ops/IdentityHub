//go:build integration

package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	user, err := repository.CreateUser(ctx, store.CreateUserParams{OrganizationID: store.ContabilidadOrganizationID, Email: email, PasswordHash: "$argon2id$placeholder", DisplayName: "Hub"})
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
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'contabilidad.senior'`, user.ID); err != nil {
		t.Fatal(err)
	}
	clients, err := repository.ListOAuthClients(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080"}
	server := NewServer(nil, "test", nil)
	server.SetOAuthService(oauth.New(repository.OAuthRepository(), clients, nil, time.Now), repository.OAuthRepository(), "http://identityhub.localhost:8080/login", time.Hour)
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

func TestRF017_CuentaBloqueadaNoAutorizaConSesionHubExistente(t *testing.T) {
	fixture := newHubFixture(t, "hub-locked@example.test")
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE users SET status = 'locked' WHERE id = $1`, fixture.userID); err != nil {
		t.Fatal(err)
	}
	var retainedHubSessions int
	if err := fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM hub_sessions WHERE user_id = $1 AND revoked_at IS NULL`, fixture.userID).Scan(&retainedHubSessions); err != nil || retainedHubSessions != 1 {
		t.Fatalf("unrevoked hub sessions=%d err=%v, want the existing session retained while locked", retainedHubSessions, err)
	}
	if fixture.authorize(t) {
		t.Fatal("locked user hub session must redirect to login")
	}
}

type resetPublisher struct{}

func (resetPublisher) Publish(context.Context, string, any) error { return nil }

type resetHasher struct{}

func (resetHasher) Hash(value string) (string, error) { return password.Hash(value) }

// Composed through Server.Routes() with the real store: a Contabilidad user
// asking for Inventario is sent back to Inventario's registered redirect URI
// with access_denied and the original state, without a code.
func TestRF022_AuthorizeRechazaUsuarioDeOtraEmpresaConAccessDeniedYAudita(t *testing.T) {
	fixture := newHubFixture(t, "hub-cross@example.test")
	inventario := oauth.Client{ID: "inventario", RedirectURI: "http://inventario.localhost:8080/oauth/callback"}
	request := httptest.NewRequest(http.MethodGet, oauthAuthorizePath(inventario), nil)
	request.AddCookie(&http.Cookie{Name: hubSessionCookieName, Value: fixture.hubCookie})
	response := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Scheme+"://"+location.Host+location.Path != inventario.RedirectURI || location.Query().Get("error") != "access_denied" || location.Query().Get("state") != "state" || location.Query().Get("code") != "" {
		t.Fatalf("location=%s", location)
	}
	var codes, audits int
	if err := fixture.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM authorization_codes WHERE user_id = $1), (SELECT count(*) FROM audit_log WHERE action = 'oauth_authorization_rejected' AND actor_user_id = $1 AND metadata->>'reason' = 'organization_mismatch' AND metadata->>'client_id' = 'inventario')`, fixture.userID).Scan(&codes, &audits); err != nil {
		t.Fatal(err)
	}
	if codes != 0 || audits != 1 {
		t.Fatalf("codes=%d audits=%d, want 0 and 1", codes, audits)
	}
}
