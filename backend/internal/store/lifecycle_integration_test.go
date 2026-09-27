//go:build integration

package store_test

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRNF005_NewAbreUnPoolRealYVerificaConexionConPing(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	// New only accepts a DSN, not a pre-built pool, so this rebuilds a plain
	// connection string for the disposable database testdb.New already created
	// and migrated instead of provisioning a second database just for this test.
	pool := testdb.New(t)
	cfg := pool.Config().ConnConfig
	dsn := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:     cfg.Database,
		RawQuery: "sslmode=disable",
	}).String()

	ctx := context.Background()
	repository, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer repository.Close()

	if repository.Pool() == nil {
		t.Fatal("Pool() = nil after New")
	}
	if err := repository.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestRNF005_NewRechazaUnDSNMalformado(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	testdb.New(t) // keeps this test gated behind TEST_DATABASE_URL, like every other integration test

	if _, err := store.New(context.Background(), "://not-a-valid-dsn"); err == nil {
		t.Fatal("New(malformed DSN) = nil error, want a DSN parse error")
	}
}

func TestRNF005_NewDevuelveErrorSiLaBaseNoResponde(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	testdb.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Port 1 is a privileged, normally unbound port: the connection is refused
	// immediately instead of hanging until New's internal 5s ping timeout.
	if _, err := store.New(ctx, "postgres://127.0.0.1:1/identity?sslmode=disable"); err == nil {
		t.Fatal("New(unreachable host) = nil error, want a connectivity error")
	}
}
