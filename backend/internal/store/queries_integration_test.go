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

	created, err := queries.CreateUser(ctx, store.CreateUserParams{
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
	if auditEvent.ActorUserID == nil || *auditEvent.ActorUserID != created.ID {
		t.Errorf("InsertAuditEvent actor ID = %v, want %v", auditEvent.ActorUserID, created.ID)
	}
	if auditEvent.ResourceType == nil || *auditEvent.ResourceType != resourceType {
		t.Errorf("InsertAuditEvent resource type = %v, want %q", auditEvent.ResourceType, resourceType)
	}
	if auditEvent.ResourceID == nil || *auditEvent.ResourceID != resourceID {
		t.Errorf("InsertAuditEvent resource ID = %v, want %q", auditEvent.ResourceID, resourceID)
	}
	if auditEvent.IP == nil || *auditEvent.IP != clientIP {
		t.Errorf("InsertAuditEvent IP = %v, want %v", auditEvent.IP, clientIP)
	}
	if auditEvent.UserAgent == nil || *auditEvent.UserAgent != userAgent {
		t.Errorf("InsertAuditEvent user agent = %v, want %q", auditEvent.UserAgent, userAgent)
	}
	var gotMetadata, wantMetadata any
	if err := json.Unmarshal(auditEvent.Metadata, &gotMetadata); err != nil {
		t.Fatalf("decode returned audit metadata: %v", err)
	}
	if err := json.Unmarshal(metadata, &wantMetadata); err != nil {
		t.Fatalf("decode expected audit metadata: %v", err)
	}
	if !reflect.DeepEqual(gotMetadata, wantMetadata) {
		t.Errorf("InsertAuditEvent metadata = %s, want %s", auditEvent.Metadata, metadata)
	}

	_, err = queries.GetUserByEmail(ctx, "' OR '1'='1' --")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("injection-shaped email error = %v, want pgx.ErrNoRows", err)
	}
}
