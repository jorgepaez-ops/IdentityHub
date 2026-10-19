//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRNF011_ConsultasBaseSqlcUsanParametros(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}

	pool := testdb.New(t)
	queries, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("create store from test pool: %v", err)
	}
	ctx := context.Background()

	created, err := queries.CreateUser(ctx, store.CreateUserParams{OrganizationID: store.ContabilidadOrganizationID,
		Email:        "User@Example.test",
		PasswordHash: "$argon2id$fixed-test-value",
		DisplayName:  "Example user",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	byEmail, err := queries.GetUserByEmail(ctx, "user@example.test")
	if err != nil {
		t.Fatalf("get user by case-insensitive email: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Errorf("GetUserByEmail ID = %v, want %v", byEmail.ID, created.ID)
	}

	byID, err := queries.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get user by ID: %v", err)
	}
	if byID.Email != created.Email {
		t.Errorf("GetUserByID email = %q, want %q", byID.Email, created.Email)
	}

	resourceType := "user"
	resourceID := created.ID.String()
	userAgent := "identity-hub-integration-test"
	clientIP := netip.MustParseAddr("203.0.113.24")
	metadata := json.RawMessage(`{"source":"integration"}`)
	auditEvent, err := queries.InsertAuditEvent(ctx, store.InsertAuditEventParams{
		ActorUserID:  &created.ID,
		Action:       "user.created",
		ResourceType: &resourceType,
		ResourceID:   &resourceID,
		IP:           &clientIP,
		UserAgent:    &userAgent,
		Metadata:     metadata,
	})
	if err != nil {
		t.Fatalf("insert audit event: %v", err)
	}
	if auditEvent.Action != "user.created" {
		t.Errorf("InsertAuditEvent action = %q, want %q", auditEvent.Action, "user.created")
	}
	requirePointee(t, "InsertAuditEvent actor ID", auditEvent.ActorUserID, created.ID)
	requirePointee(t, "InsertAuditEvent resource type", auditEvent.ResourceType, resourceType)
	requirePointee(t, "InsertAuditEvent resource ID", auditEvent.ResourceID, resourceID)
	requirePointee(t, "InsertAuditEvent IP", auditEvent.IP, clientIP)
	requirePointee(t, "InsertAuditEvent user agent", auditEvent.UserAgent, userAgent)
	requireSameJSON(t, "InsertAuditEvent metadata", auditEvent.Metadata, metadata)

	_, err = queries.GetUserByEmail(ctx, "' OR '1'='1' --")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("injection-shaped email error = %v, want pgx.ErrNoRows", err)
	}
}

// requirePointee reports (without stopping the test) unless got is non-nil and points to want.
func requirePointee[T comparable](t *testing.T, label string, got *T, want T) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// requireSameJSON reports (without stopping the test) unless got and want decode to equal values.
func requireSameJSON(t *testing.T, label string, got, want json.RawMessage) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode returned audit metadata: %v", err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("decode expected audit metadata: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("%s = %s, want %s", label, got, want)
	}
}
