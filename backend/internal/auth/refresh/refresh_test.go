package refresh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type repositoryStub struct {
	rotation Rotation
	rotated  []RefreshToken
	revoked  []uuid.UUID
	audits   []AuditEvent
}

func (r *repositoryStub) WithinRefreshTransaction(_ context.Context, fn func(Writer) error) error {
	return fn(r)
}
func (r *repositoryStub) RotateRefreshToken(_ context.Context, hash []byte, next RefreshToken) (Rotation, error) {
	if string(hash) == "error" {
		return Rotation{}, errors.New("database unavailable")
	}
	if bytes.Equal(hash, hashRefreshToken("missing-token")) {
		return Rotation{}, pgx.ErrNoRows
	}
	r.rotated = append(r.rotated, next)
	return r.rotation, nil
}
func (r *repositoryStub) RevokeRefreshFamily(_ context.Context, familyID uuid.UUID) (int64, error) {
	r.revoked = append(r.revoked, familyID)
	return 2, nil
}
func (r *repositoryStub) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	return r.rotation.Roles, nil
}

type failingPublisher struct{ err error }

func (p *failingPublisher) PublishSecurityEvent(context.Context, SecurityEvent) error {
	return p.err
}

func (r *repositoryStub) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

func TestRF005_RenovarEmiteParDistintoEInvalidaElAnterior(t *testing.T) {
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	userID, familyID := uuid.New(), uuid.New()
	repo := &repositoryStub{rotation: Rotation{Status: RotationSucceeded, UserID: userID, FamilyID: familyID, Roles: []string{"user"}}}
	service := New(repo, signer, time.Hour)
	result, err := service.Refresh(context.Background(), Input{RefreshToken: "old-token"})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" || result.RefreshToken == "old-token" {
		t.Fatalf("result = %+v", result)
	}
	if len(repo.rotated) != 1 || len(repo.rotated[0].TokenHash) != sha256.Size {
		t.Fatalf("rotated=%+v", repo.rotated)
	}
	if len(repo.revoked) != 0 {
		t.Fatalf("family must not be revoked: %v", repo.revoked)
	}
}

func TestRF006_ReusoDeTokenRotadoRevocaLaFamilia(t *testing.T) {
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	familyID := uuid.New()
	ip := netip.MustParseAddr("203.0.113.10")
	repo := &repositoryStub{rotation: Rotation{Status: RotationReused, UserID: uuid.New(), FamilyID: familyID}}
	_, err = New(repo, signer, time.Hour).Refresh(context.Background(), Input{RefreshToken: "reused", IP: &ip})
	if !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("Refresh error=%v", err)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != familyID {
		t.Fatalf("revoked=%v", repo.revoked)
	}
	if len(repo.audits) != 1 || repo.audits[0].Action != "refresh_reuse_detected" || repo.audits[0].IP == nil || *repo.audits[0].IP != ip {
		t.Fatalf("audits=%+v", repo.audits)
	}
}

func TestRF006_ReusoConFalloDePublicacionSigueRevocandoYDevuelveReuso(t *testing.T) {
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	familyID := uuid.New()
	repo := &repositoryStub{rotation: Rotation{Status: RotationReused, UserID: uuid.New(), FamilyID: familyID}}
	service := New(repo, signer, time.Hour).WithEventPublisher(&failingPublisher{err: errors.New("broker unavailable")})
	_, err = service.Refresh(context.Background(), Input{RefreshToken: "reused"})
	if !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("Refresh error=%v, want ErrRefreshReuse even when publishing fails", err)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != familyID {
		t.Fatalf("revoked=%v, family must stay revoked despite the publish failure", repo.revoked)
	}
}

func TestRF005_TokenInexistenteDevuelveInvalido(t *testing.T) {
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	repo := &repositoryStub{}
	_, err = New(repo, signer, time.Hour).Refresh(context.Background(), Input{RefreshToken: "missing-token"})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Refresh error=%v, want ErrInvalidRefreshToken", err)
	}
}

func TestRF005_HashRefreshToken(t *testing.T) {
	got := hashRefreshToken("opaque")
	want := sha256.Sum256([]byte("opaque"))
	if string(got) != string(want[:]) {
		t.Fatal("refresh token must be SHA-256 hashed")
	}
}
