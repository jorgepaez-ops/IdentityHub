package session

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repositoryStub struct {
	sessions []Session
	revoked  []uuid.UUID
	audits   []AuditEvent
	err      error
}

func (r *repositoryStub) ListActiveSessions(context.Context, uuid.UUID) ([]Session, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.sessions, nil
}
func (r *repositoryStub) WithinSessionTransaction(_ context.Context, fn func(Writer) error) error {
	return fn(r)
}
func (r *repositoryStub) RevokeActiveSession(_ context.Context, _ uuid.UUID, familyID uuid.UUID) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	for _, item := range r.sessions {
		if item.ID == familyID {
			r.revoked = append(r.revoked, familyID)
			return true, nil
		}
	}
	return false, nil
}
func (r *repositoryStub) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	if r.err != nil {
		return r.err
	}
	r.audits = append(r.audits, event)
	return nil
}

func TestRF016_ListarDevuelveLasFamiliasActivas(t *testing.T) {
	userID := uuid.New()
	familyID := uuid.New()
	ip := netip.MustParseAddr("203.0.113.7")
	now := time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC)
	repository := &repositoryStub{sessions: []Session{{ID: familyID, IP: &ip, UserAgent: "Identity Hub test", CreatedAt: now.Add(-time.Hour), LastUsedAt: now}}}

	items, err := New(repository).List(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != familyID || items[0].IP == nil || *items[0].IP != ip || items[0].UserAgent != "Identity Hub test" || !items[0].CreatedAt.Equal(now.Add(-time.Hour)) || !items[0].LastUsedAt.Equal(now) {
		t.Fatalf("sessions=%+v", items)
	}
}

func TestRF016_RevocarSoloLaFamiliaPropiaYLaAudita(t *testing.T) {
	userID := uuid.New()
	familyID := uuid.New()
	repository := &repositoryStub{sessions: []Session{{ID: familyID}}}

	if err := New(repository).Revoke(context.Background(), userID, familyID); err != nil {
		t.Fatal(err)
	}
	if len(repository.revoked) != 1 || repository.revoked[0] != familyID {
		t.Fatalf("revoked=%v", repository.revoked)
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "session_revoked" || repository.audits[0].ResourceID != familyID.String() || repository.audits[0].ActorUserID == nil || *repository.audits[0].ActorUserID != userID {
		t.Fatalf("audits=%+v", repository.audits)
	}
}

func TestRF016_RevocarDesconocidaNoDistinguePropiedad(t *testing.T) {
	err := New(&repositoryStub{}).Revoke(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err=%v, want ErrSessionNotFound", err)
	}
}
