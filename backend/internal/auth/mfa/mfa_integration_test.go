//go:build integration

package mfa_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/auth/lockout"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
	"github.com/jorgepaez/identity-hub/internal/testsession"
)

type publisher struct{}

func (publisher) Publish(context.Context, string, any) error { return nil }

func newActiveUser(t *testing.T, pool *pgxpool.Pool, repository *store.Store, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: email, PasswordHash: hash, DisplayName: "MFA"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func newSigner(t *testing.T) *token.Service {
	t.Helper()
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// wrongCode returns a well-formed code different from the mailed one.
func wrongCode(mailed string) string {
	if mailed == "000000" {
		return "000001"
	}
	return "000000"
}

func TestRF014_CodigoMFASeConsumeUnaSolaVezEnPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userID := newActiveUser(t, pool, repository, "mfa@example.test")
	rawToken, code := []byte("mfa-integration-token"), "123456"
	tokenHash := sha256.Sum256(rawToken)
	mac := hmac.New(sha256.New, rawToken)
	mac.Write([]byte(code))
	if _, err := pool.Exec(ctx, `INSERT INTO mfa_challenges (user_id, token_hash, code_hash, expires_at, attempts_left) VALUES ($1,$2,$3,$4,5)`, userID, tokenHash[:], mac.Sum(nil), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	service := mfa.New(repository, publisher{}, bytes.NewReader(bytes.Repeat([]byte{1}, 64)), time.Now).WithTokenService(newSigner(t), time.Hour)
	encoded := base64.RawURLEncoding.EncodeToString(rawToken)
	if _, err := service.Verify(ctx, mfa.VerifyInput{Token: encoded, Code: code}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Verify(ctx, mfa.VerifyInput{Token: encoded, Code: code}); err == nil {
		t.Fatal("reused MFA challenge was accepted")
	}
}

// RF-014 + RF-017: guessing codes across successive challenges, knowing the
// password, must reach the same lockout as guessing the password.
func TestRF017_CodigosMFAIncorrectosEnDesafiosSucesivosBloqueanLaCuenta(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userID := newActiveUser(t, pool, repository, "mfa-lockout@example.test")
	session := testsession.New(repository, newSigner(t), lockout.Config{AccountMaxFailures: 3, IPMaxFailures: 50, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute})
	ip := netip.MustParseAddr("198.51.100.7")
	userAgent := "code-guesser/1.0"
	input := login.Input{Email: "mfa-lockout@example.test", Password: "correct horse battery", IP: &ip, UserAgent: &userAgent}

	for attempt := 1; attempt <= 3; attempt++ {
		challenge, err := session.Login.Login(ctx, input)
		if err != nil {
			t.Fatalf("attempt %d: password step: %v", attempt, err)
		}
		_, err = session.MFA.Verify(ctx, mfa.VerifyInput{Token: challenge.MfaToken, Code: wrongCode(session.Mailbox.Code()), IP: &ip, UserAgent: &userAgent})
		if !errors.Is(err, mfa.ErrCodeInvalid) {
			t.Fatalf("attempt %d: verify error = %v, want ErrCodeInvalid", attempt, err)
		}
	}

	var status string
	var lockedUntil *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, locked_until FROM users WHERE id = $1`, userID).Scan(&status, &lockedUntil); err != nil {
		t.Fatal(err)
	}
	if status != "locked" || lockedUntil == nil || !lockedUntil.After(time.Now().Add(14*time.Minute)) {
		t.Fatalf("account after three rejected codes: status=%q locked_until=%v", status, lockedUntil)
	}
	var lockAudits int
	var lockIP, lockAgent string
	if err := pool.QueryRow(ctx, `SELECT count(*), coalesce(max(host(ip)), ''), coalesce(max(user_agent), '') FROM audit_log WHERE actor_user_id = $1 AND action = 'account_locked'`, userID).Scan(&lockAudits, &lockIP, &lockAgent); err != nil {
		t.Fatal(err)
	}
	if lockAudits != 1 || lockIP != ip.String() || lockAgent != userAgent {
		t.Fatalf("account_locked audit: count=%d ip=%q user-agent=%q", lockAudits, lockIP, lockAgent)
	}
	var rejected int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'mfa_code_rejected' AND ip = $2 AND user_agent = $3`, userID, ip, userAgent).Scan(&rejected); err != nil {
		t.Fatal(err)
	}
	if rejected != 3 {
		t.Fatalf("mfa_code_rejected rows with IP and user-agent = %d, want 3", rejected)
	}
	if locked := session.Mailbox.LockedEvents(); len(locked) != 1 || locked[0].Data.FailedAttempts != 3 || locked[0].Data.Email != "mfa-lockout@example.test" {
		t.Fatalf("security.account_locked events = %+v", locked)
	}

	if _, err := session.Login.Login(ctx, input); !errors.Is(err, login.ErrAccountLocked) {
		t.Fatalf("password step on locked account = %v, want ErrAccountLocked", err)
	}
	// A challenge that was already open when the lock landed no longer verifies,
	// even with its correct code.
	fresh, err := session.MFA.Issue(ctx, mfa.User{ID: userID, Email: "mfa-lockout@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.MFA.Verify(ctx, mfa.VerifyInput{Token: fresh.Token, Code: session.Mailbox.Code()}); !errors.Is(err, mfa.ErrChallengeInvalid) {
		t.Fatalf("correct code on a locked account = %v, want ErrChallengeInvalid", err)
	}
	var sessions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1`, userID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatalf("locked account obtained %d refresh sessions", sessions)
	}
}

// Rejected MFA codes also feed the per-IP limit (RF-017).
func TestRF017_RechazosMFACuentanParaElLimitePorIP(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	newActiveUser(t, pool, repository, "mfa-ip@example.test")
	session := testsession.New(repository, newSigner(t), lockout.Config{AccountMaxFailures: 50, IPMaxFailures: 2, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute})
	ip := netip.MustParseAddr("198.51.100.8")
	input := login.Input{Email: "mfa-ip@example.test", Password: "correct horse battery", IP: &ip}

	for attempt := 1; attempt <= 2; attempt++ {
		challenge, err := session.Login.Login(ctx, input)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		if _, err := session.MFA.Verify(ctx, mfa.VerifyInput{Token: challenge.MfaToken, Code: wrongCode(session.Mailbox.Code()), IP: &ip}); !errors.Is(err, mfa.ErrCodeInvalid) {
			t.Fatalf("attempt %d: verify error = %v", attempt, err)
		}
	}
	if _, err := session.Login.Login(ctx, input); !errors.Is(err, login.ErrIPRateLimited) {
		t.Fatalf("login after two rejected codes from one IP = %v, want ErrIPRateLimited", err)
	}
}

func newIntegrationService(t *testing.T, repository *store.Store, now func() time.Time) *mfa.Service {
	t.Helper()
	return mfa.New(repository, publisher{}, nil, now).WithTokenService(newSigner(t), time.Hour)
}

func TestRF014_CreateChallengeGuardaLosIntentosRecibidosEnPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userID := newActiveUser(t, pool, repository, "mfa-attempts@example.test")
	hash := bytes.Repeat([]byte{3}, sha256.Size)
	if err := repository.WithinMFATransaction(ctx, func(w mfa.Writer) error {
		return w.CreateChallenge(ctx, mfa.CreateParams{UserID: userID, TokenHash: hash, CodeHash: hash, ExpiresAt: time.Now().Add(time.Minute), AttemptsLeft: 3, SentAt: time.Now()})
	}); err != nil {
		t.Fatal(err)
	}
	var attempts int
	if err := pool.QueryRow(ctx, `SELECT attempts_left FROM mfa_challenges WHERE user_id = $1`, userID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts_left = %d, want the 3 passed in", attempts)
	}
}

func TestRF014_UnDesafioNuevoAnulaLosAbiertosYLaEmisionTieneTope(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user := mfa.User{ID: newActiveUser(t, pool, repository, "mfa-limit@example.test"), Email: "mfa-limit@example.test"}
	service := newIntegrationService(t, repository, time.Now)

	var first mfa.Challenge
	for issued := 1; issued <= mfa.MaxIssuancesPerWindow; issued++ {
		challenge, err := service.Issue(ctx, user)
		if err != nil {
			t.Fatalf("issue %d: %v", issued, err)
		}
		if issued == 1 {
			first = challenge
		}
	}
	if _, err := service.Issue(ctx, user); !errors.Is(err, mfa.ErrIssuanceLimited) {
		t.Fatalf("issue beyond the limit = %v, want ErrIssuanceLimited", err)
	}
	var total, open int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE used_at IS NULL) FROM mfa_challenges WHERE user_id = $1`, user.ID).Scan(&total, &open); err != nil {
		t.Fatal(err)
	}
	if total != mfa.MaxIssuancesPerWindow || open != 1 {
		t.Fatalf("challenges total=%d open=%d, want %d total and only the latest open", total, open, mfa.MaxIssuancesPerWindow)
	}
	// The superseded first challenge no longer verifies, even with its own code.
	if _, err := service.Verify(ctx, mfa.VerifyInput{Token: first.Token, Code: "000000"}); !errors.Is(err, mfa.ErrChallengeInvalid) {
		t.Fatalf("verify on a superseded challenge = %v, want ErrChallengeInvalid", err)
	}
}

func TestRF014_LaEmisionFueraDeLaVentanaNoCuenta(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user := mfa.User{ID: newActiveUser(t, pool, repository, "mfa-window@example.test"), Email: "mfa-window@example.test"}
	past := time.Now().Add(-2 * mfa.IssuanceWindow)
	for issued := 0; issued < mfa.MaxIssuancesPerWindow; issued++ {
		if _, err := newIntegrationService(t, repository, func() time.Time { return past }).Issue(ctx, user); err != nil {
			t.Fatalf("old issue %d: %v", issued, err)
		}
	}
	if _, err := newIntegrationService(t, repository, time.Now).Issue(ctx, user); err != nil {
		t.Fatalf("issue after the window elapsed: %v", err)
	}
}

func TestRF014_ElReenvioGuardaLastSentAtConElRelojDelServicio(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userID := newActiveUser(t, pool, repository, "mfa-resend@example.test")
	start := time.Now().UTC().Truncate(time.Microsecond)
	challenge, err := newIntegrationService(t, repository, func() time.Time { return start }).Issue(ctx, mfa.User{ID: userID, Email: "mfa-resend@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	var sentAt time.Time
	if err := pool.QueryRow(ctx, `SELECT last_sent_at FROM mfa_challenges WHERE user_id = $1`, userID).Scan(&sentAt); err != nil {
		t.Fatal(err)
	}
	if !sentAt.Equal(start) {
		t.Fatalf("last_sent_at after issue = %v, want the service clock %v", sentAt, start)
	}
	later := start.Add(mfa.ResendInterval + time.Second)
	if err := newIntegrationService(t, repository, func() time.Time { return later }).Resend(ctx, challenge.Token); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT last_sent_at FROM mfa_challenges WHERE user_id = $1`, userID).Scan(&sentAt); err != nil {
		t.Fatal(err)
	}
	if !sentAt.Equal(later) {
		t.Fatalf("last_sent_at after resend = %v, want the service clock %v", sentAt, later)
	}
}
