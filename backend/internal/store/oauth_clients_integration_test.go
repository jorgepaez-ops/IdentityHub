//go:build integration

package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF020_PostgresListaLosClientesRegistradosConSuEmpresa(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	repository, err := store.NewWithPool(testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	clients, err := repository.ListOAuthClients(ctx)
	if err != nil {
		t.Fatalf("ListOAuthClients() error = %v", err)
	}
	want := []oauth.Client{
		{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback", Origin: "http://contabilidad.localhost:8080", OrganizationID: store.ContabilidadOrganizationID},
		{ID: "inventario", RedirectURI: "http://inventario.localhost:8080/oauth/callback", Origin: "http://inventario.localhost:8080", OrganizationID: store.EmpresaBOrganizationID},
	}
	if !reflect.DeepEqual(clients, want) {
		t.Fatalf("clients = %+v, want %+v", clients, want)
	}
	if err := oauth.ValidateClients(clients); err != nil {
		t.Fatalf("seeded clients are not a valid registration: %v", err)
	}
}

func TestRF022_PostgresAutorizaSoloAUsuariosDeLaEmpresaConRolDeLaAplicacion(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := repository.ListOAuthClients(ctx)
	if err != nil {
		t.Fatal(err)
	}
	service := oauth.New(repository.OAuthRepository(), clients, &roleTestReader{}, time.Now)

	contab := createOrganizationUser(t, ctx, repository, store.ContabilidadOrganizationID, "contab-ok@example.test")
	contabSinRol := createOrganizationUser(t, ctx, repository, store.ContabilidadOrganizationID, "contab-sin-rol@example.test")
	empresaB := createOrganizationUser(t, ctx, repository, store.EmpresaBOrganizationID, "b-ok@example.test")
	empresaBSinRol := createOrganizationUser(t, ctx, repository, store.EmpresaBOrganizationID, "b-sin-rol@example.test")
	for user, roleName := range map[uuid.UUID]string{contab.ID: "contabilidad.senior", empresaB.ID: "inventario.bodeguero"} {
		if err := assignRoleSQL(ctx, pool, user, roleName); err != nil {
			t.Fatal(err)
		}
	}
	if err := assignRoleSQL(ctx, pool, empresaB.ID, "inventario.consulta"); err != nil {
		t.Fatal(err)
	}
	if err := assignRoleSQL(ctx, pool, empresaB.ID, "user"); err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name       string
		userID     uuid.UUID
		clientID   string
		wantReason string
	}{
		{"usuario de Contabilidad en Contabilidad", contab.ID, "contabilidad", ""},
		{"usuario de Empresa B en Inventario", empresaB.ID, "inventario", ""},
		{"usuario de Contabilidad en Inventario", contab.ID, "inventario", oauth.RejectionOrganizationMismatch},
		{"usuario de Empresa B en Contabilidad", empresaB.ID, "contabilidad", oauth.RejectionOrganizationMismatch},
		{"misma empresa sin rol en Contabilidad", contabSinRol.ID, "contabilidad", oauth.RejectionNoApplicationRole},
		{"misma empresa sin rol en Inventario", empresaBSinRol.ID, "inventario", oauth.RejectionNoApplicationRole},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client := clientByID(t, clients, testCase.clientID)
			before := countRows(t, ctx, pool, `SELECT count(*) FROM authorization_codes WHERE user_id = $1`, testCase.userID)
			_, err := service.Authorize(ctx, testCase.userID, oauth.AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, ResponseType: "code", State: "state", CodeChallenge: challengeForIntegration("verifier"), CodeChallengeMethod: "S256"})
			after := countRows(t, ctx, pool, `SELECT count(*) FROM authorization_codes WHERE user_id = $1`, testCase.userID)
			if testCase.wantReason == "" {
				if err != nil || after != before+1 {
					t.Fatalf("Authorize() error = %v, codes %d -> %d, want one new code", err, before, after)
				}
				return
			}
			var rejected *oauth.RejectedError
			if !errors.As(err, &rejected) || rejected.Reason != testCase.wantReason {
				t.Fatalf("Authorize() error = %v, want rejection %q", err, testCase.wantReason)
			}
			if after != before {
				t.Fatalf("a rejected authorization stored a code (%d -> %d)", before, after)
			}
			var reason, clientID string
			if err := pool.QueryRow(ctx, `SELECT metadata->>'reason', metadata->>'client_id' FROM audit_log WHERE action = 'oauth_authorization_rejected' AND actor_user_id = $1`, testCase.userID).Scan(&reason, &clientID); err != nil {
				t.Fatalf("rejection audit event: %v", err)
			}
			if reason != testCase.wantReason || clientID != testCase.clientID {
				t.Fatalf("audit reason=%q client_id=%q, want %q / %q", reason, clientID, testCase.wantReason, testCase.clientID)
			}
		})
	}

	t.Run("el token de Inventario lleva solo roles y permisos de Inventario", func(t *testing.T) {
		client := clientByID(t, clients, "inventario")
		issued, err := service.Authorize(ctx, empresaB.ID, oauth.AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, ResponseType: "code", State: "state", CodeChallenge: challengeForIntegration("verifier"), CodeChallengeMethod: "S256"})
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Exchange(ctx, oauth.ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"})
		if err != nil {
			t.Fatal(err)
		}
		if result.ClientID != "inventario" {
			t.Fatalf("ClientID = %q, want inventario", result.ClientID)
		}
		if want := []string{"inventario.bodeguero", "inventario.consulta"}; !reflect.DeepEqual(result.Roles, want) {
			t.Fatalf("roles = %v, want %v", result.Roles, want)
		}
		if want := []string{"movimientos.registrar", "productos.editar", "productos.ver"}; !reflect.DeepEqual(result.Permissions, want) {
			t.Fatalf("permissions = %v, want %v", result.Permissions, want)
		}
	})
}

func clientByID(t *testing.T, clients []oauth.Client, id string) oauth.Client {
	t.Helper()
	for _, client := range clients {
		if client.ID == id {
			return client
		}
	}
	t.Fatalf("client %q is not registered", id)
	return oauth.Client{}
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
