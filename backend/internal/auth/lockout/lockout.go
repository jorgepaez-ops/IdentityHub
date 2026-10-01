// Package lockout holds the RF-017 account lockout policy shared by every
// authentication step that can reject a credential (password and email MFA
// code), so their thresholds cannot drift apart.
package lockout

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	defaultAccountMaxFailures = 5
	defaultIPMaxFailures      = 20
	defaultFailureWindow      = 15 * time.Minute
	defaultLockoutDuration    = 15 * time.Minute
)

// Config controls the independent account and IP sliding-window limits.
type Config struct {
	AccountMaxFailures int
	IPMaxFailures      int
	FailureWindow      time.Duration
	LockoutDuration    time.Duration
}

// Default returns the policy used when composition injects none.
func Default() Config {
	return Config{
		AccountMaxFailures: defaultAccountMaxFailures,
		IPMaxFailures:      defaultIPMaxFailures,
		FailureWindow:      defaultFailureWindow,
		LockoutDuration:    defaultLockoutDuration,
	}
}

// Valid reports whether every limit is positive.
func (c Config) Valid() bool {
	return c.AccountMaxFailures > 0 && c.IPMaxFailures > 0 && c.FailureWindow > 0 && c.LockoutDuration > 0
}

// Store is the persistence surface the account policy needs.
type Store interface {
	CountLoginFailuresByAccount(context.Context, uuid.UUID, time.Time) (int64, error)
	LockLoginUser(context.Context, uuid.UUID, time.Time) error
}

// Outcome carries the details security.account_locked needs, matching the
// AsyncAPI contract (lockedUntil and failedAttempts).
type Outcome struct {
	Locked         bool
	LockedUntil    time.Time
	FailedAttempts int
}

// EvaluateAccount counts the failures already recorded in the window and, once
// they reach the threshold, locks the account. The caller audits
// account_locked and publishes the event after its transaction commits.
func (c Config) EvaluateAccount(ctx context.Context, store Store, userID uuid.UUID, now time.Time) (Outcome, error) {
	failures, err := store.CountLoginFailuresByAccount(ctx, userID, now.Add(-c.FailureWindow))
	if err != nil {
		return Outcome{}, fmt.Errorf("count login failures by account: %w", err)
	}
	if failures < int64(c.AccountMaxFailures) {
		return Outcome{}, nil
	}
	lockedUntil := now.Add(c.LockoutDuration)
	if err := store.LockLoginUser(ctx, userID, lockedUntil); err != nil {
		return Outcome{}, fmt.Errorf("lock login user: %w", err)
	}
	return Outcome{Locked: true, LockedUntil: lockedUntil, FailedAttempts: int(failures)}, nil
}
