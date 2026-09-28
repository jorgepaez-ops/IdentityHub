package lockout

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	failures    int64
	sinceSeen   time.Time
	lockedUntil time.Time
	locked      bool
}

func (s *fakeStore) CountLoginFailuresByAccount(_ context.Context, _ uuid.UUID, since time.Time) (int64, error) {
	s.sinceSeen = since
	return s.failures, nil
}

func (s *fakeStore) LockLoginUser(_ context.Context, _ uuid.UUID, until time.Time) error {
	s.locked, s.lockedUntil = true, until
	return nil
}

func TestRF017_EvaluateAccountBloqueaAlAlcanzarElUmbral(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	cfg := Config{AccountMaxFailures: 3, IPMaxFailures: 9, FailureWindow: 10 * time.Minute, LockoutDuration: 20 * time.Minute}

	below := &fakeStore{failures: 2}
	outcome, err := cfg.EvaluateAccount(context.Background(), below, uuid.New(), now)
	if err != nil || outcome.Locked || below.locked {
		t.Fatalf("below threshold: outcome=%+v err=%v locked=%t", outcome, err, below.locked)
	}
	if !below.sinceSeen.Equal(now.Add(-10 * time.Minute)) {
		t.Fatalf("window start = %v", below.sinceSeen)
	}

	at := &fakeStore{failures: 3}
	outcome, err = cfg.EvaluateAccount(context.Background(), at, uuid.New(), now)
	if err != nil || !outcome.Locked || outcome.FailedAttempts != 3 || !outcome.LockedUntil.Equal(now.Add(20*time.Minute)) || !at.locked {
		t.Fatalf("at threshold: outcome=%+v err=%v store=%+v", outcome, err, at)
	}
}

func TestRF017_ConfigValidaExigeTodosLosLimitesPositivos(t *testing.T) {
	if !Default().Valid() {
		t.Fatal("default lockout config must be valid")
	}
	if (Config{}).Valid() {
		t.Fatal("zero lockout config must be invalid")
	}
}
