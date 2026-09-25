package logout

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type repositoryStub struct {
	userID  uuid.UUID
	err     error
	revoked [][]byte
	audits  []AuditEvent
}

func (r *repositoryStub) WithinLogoutTransaction(_ context.Context, fn func(Writer) error) error {
	return fn(r)
}

func (r *repositoryStub) RevokeRefreshToken(_ context.Context, hash []byte) (uuid.UUID, error) {
	if r.err != nil {
		return uuid.Nil, r.err
	}
	r.revoked = append(r.revoked, hash)
	return r.userID, nil
}

func (r *repositoryStub) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

func TestRF007_LogoutRevocaElTokenYRegistraAuditoria(t *testing.T) {
	userID := uuid.New()
	ip := netip.MustParseAddr("203.0.113.10")
	repo := &repositoryStub{userID: userID}
	token := base64.RawURLEncoding.EncodeToString([]byte("current-refresh-raw-bytes"))
	if err := New(repo).Logout(context.Background(), Input{RefreshToken: token, IP: &ip}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	wantHash := sha256.Sum256([]byte("current-refresh-raw-bytes"))
	if len(repo.revoked) != 1 || string(repo.revoked[0]) != string(wantHash[:]) {
		t.Fatalf("revoked=%v", repo.revoked)
	}
	if len(repo.audits) != 1 || repo.audits[0].Action != "logout" || repo.audits[0].ActorUserID == nil || *repo.audits[0].ActorUserID != userID || repo.audits[0].IP == nil || *repo.audits[0].IP != ip {
		t.Fatalf("audits=%+v", repo.audits)
	}
}

func TestRF007_LogoutDeTokenDesconocidoDevuelveInvalido(t *testing.T) {
	repo := &repositoryStub{err: pgx.ErrNoRows}
	err := New(repo).Logout(context.Background(), Input{RefreshToken: "unknown"})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Logout error=%v, want ErrInvalidRefreshToken", err)
	}
	if len(repo.audits) != 0 {
		t.Fatalf("audits=%+v, want none for an invalid token", repo.audits)
	}
}

func TestRF007_LogoutSinTokenDevuelveInvalido(t *testing.T) {
	err := New(&repositoryStub{}).Logout(context.Background(), Input{})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Logout error=%v, want ErrInvalidRefreshToken", err)
	}
}
