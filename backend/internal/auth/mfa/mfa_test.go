package mfa

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
	"github.com/jorgepaez/identity-hub/internal/auth/lockout"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/events"
)

var challengeSecret = []byte("challenge-token")

func encodedToken() string { return base64.RawURLEncoding.EncodeToString(challengeSecret) }

// codeMAC is computed independently of the implementation: HMAC-SHA256 keyed
// with the raw challenge token.
func codeMAC(raw []byte, code string) []byte {
	mac := hmac.New(sha256.New, raw)
	mac.Write([]byte(code))
	return mac.Sum(nil)
}

func newSigner(t *testing.T) *token.Service {
	t.Helper()
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func activeChallenge(code string, attempts int) StoredChallenge {
	return StoredChallenge{
		ID:           uuid.New(),
		User:         User{ID: uuid.New(), Email: "ada@example.test", DisplayName: "Ada", Status: "active"},
		CodeHash:     codeMAC(challengeSecret, code),
		ExpiresAt:    time.Now().Add(time.Minute),
		LastSentAt:   time.Now().Add(-2 * ResendInterval),
		AttemptsLeft: attempts,
	}
}

func newVerifier(t *testing.T, repo *memoryRepository, publisher *fakePublisher) *Service {
	t.Helper()
	return New(repo, publisher, bytes.NewReader(bytes.Repeat([]byte{9}, 64)), time.Now).WithTokenService(newSigner(t), time.Hour)
}

func TestRF014_EmiteDesafioDeCincoMinutosYGuardaSoloHashes(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	repo := &memoryRepository{}
	publisher := &fakePublisher{}
	service := New(repo, publisher, bytes.NewReader(bytes.Repeat([]byte{7}, 64)), func() time.Time { return now })

	challenge, err := service.Issue(context.Background(), User{ID: uuid.New(), Email: "ada@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Token == "" || challenge.ExpiresIn != 300 {
		t.Fatalf("challenge=%+v", challenge)
	}
	if !repo.created.ExpiresAt.Equal(now.Add(ChallengeTTL)) || repo.created.AttemptsLeft != MaxAttempts {
		t.Fatalf("stored=%+v", repo.created)
	}
	if bytes.Equal(repo.created.TokenHash, []byte(challenge.Token)) || len(repo.created.TokenHash) != sha256.Size || len(repo.created.CodeHash) != sha256.Size {
		t.Fatalf("challenge stores raw secret or missing hash")
	}
}

func TestRF014_CodigoSeGuardaComoHMACConLaClaveDelTokenCrudo(t *testing.T) {
	repo := &memoryRepository{}
	publisher := &fakePublisher{}
	service := New(repo, publisher, bytes.NewReader(bytes.Repeat([]byte{7}, 64)), time.Now)

	challenge, err := service.Issue(context.Background(), User{ID: uuid.New(), Email: "ada@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(challenge.Token)
	if err != nil {
		t.Fatal(err)
	}
	code := publisher.lastMFA(t).Data.Code
	if !bytes.Equal(repo.created.CodeHash, codeMAC(raw, code)) {
		t.Fatal("stored code hash is not HMAC-SHA256(rawToken, code)")
	}
	plain := sha256.Sum256([]byte(code))
	if bytes.Equal(repo.created.CodeHash, plain[:]) {
		t.Fatal("stored code hash is an unkeyed SHA-256 of the six-digit code")
	}
}

func TestRF014_ReenvioGuardaHMACYRechazaCuentaNoActiva(t *testing.T) {
	for _, status := range []string{"locked", "disabled", "pending_verification"} {
		challenge := activeChallenge("123456", MaxAttempts)
		challenge.User.Status = status
		repo := &memoryRepository{challenge: challenge}
		publisher := &fakePublisher{}
		err := newVerifier(t, repo, publisher).Resend(context.Background(), encodedToken())
		if !errors.Is(err, ErrChallengeInvalid) || repo.resentHash != nil || len(publisher.published) != 0 {
			t.Fatalf("status=%s err=%v resent=%x published=%d", status, err, repo.resentHash, len(publisher.published))
		}
	}

	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts)}
	publisher := &fakePublisher{}
	if err := newVerifier(t, repo, publisher).Resend(context.Background(), encodedToken()); err != nil {
		t.Fatal(err)
	}
	code := publisher.lastMFA(t).Data.Code
	if !bytes.Equal(repo.resentHash, codeMAC(challengeSecret, code)) {
		t.Fatal("resent code hash is not HMAC-SHA256(rawToken, code)")
	}
}

func TestRF014_GeneraCodigoSinSesgoPorRechazo(t *testing.T) {
	// 0x0fffff (1048575) exceeds 999999 and must be redrawn, not reduced with a modulo.
	reader := bytes.NewReader([]byte{0x0f, 0xff, 0xff, 0x00, 0x00, 0x2a})
	code, err := generateCode(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != "000042" {
		t.Fatalf("code = %q, want the redrawn value 000042", code)
	}
}

func TestRF014_CodigoCorrectoSeConsumeYEmiteTokens(t *testing.T) {
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts)}
	result, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "123456"})
	if err != nil || result.AccessToken == "" || result.RefreshToken == "" || !repo.consumed {
		t.Fatalf("result=%+v err=%v consumed=%t", result, err, repo.consumed)
	}
	if result.ExpiresIn != token.AccessTokenExpiresIn {
		t.Fatalf("ExpiresIn = %d, want the access token lifetime %d", result.ExpiresIn, token.AccessTokenExpiresIn)
	}
}

func TestRF014_VerificacionConservaIPConfiableYAgenteEnLaSesionRefresh(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.8")
	userAgent := "Identity Hub test client"
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts)}

	_, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "123456", IP: &ip, UserAgent: &userAgent})
	if err != nil {
		t.Fatal(err)
	}
	if repo.refresh.IP == nil || *repo.refresh.IP != ip {
		t.Fatalf("refresh IP=%v, want %v", repo.refresh.IP, ip)
	}
	if repo.refresh.UserAgent == nil || *repo.refresh.UserAgent != userAgent {
		t.Fatalf("refresh user-agent=%v, want %q", repo.refresh.UserAgent, userAgent)
	}
}

func TestRF003_ElExitoDelLoginSeRegistraAlAceptarElCodigo(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.9")
	userAgent := "Identity Hub test client"
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts)}
	if _, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "123456", IP: &ip, UserAgent: &userAgent}); err != nil {
		t.Fatal(err)
	}
	if !repo.lastLoginUpdated {
		t.Fatal("last_login_at was not recorded when the code was accepted")
	}
	audit, ok := repo.audit("login_succeeded")
	if !ok || audit.IP == nil || *audit.IP != ip || audit.UserAgent == nil || *audit.UserAgent != userAgent {
		t.Fatalf("login_succeeded audit=%+v found=%t", audit, ok)
	}
}

func TestRF003_UnCodigoRechazadoNoRegistraExitoDeLogin(t *testing.T) {
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts)}
	_, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "999999"})
	if !errors.Is(err, ErrCodeInvalid) || repo.lastLoginUpdated {
		t.Fatalf("err=%v lastLoginUpdated=%t", err, repo.lastLoginUpdated)
	}
	if _, ok := repo.audit("login_succeeded"); ok {
		t.Fatal("login_succeeded was audited for a rejected code")
	}
}

func TestRF014_CodigoIncorrectoSeAuditaConIPYAgente(t *testing.T) {
	ip := netip.MustParseAddr("198.51.100.4")
	userAgent := "attacker/1.0"
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts)}
	_, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "999999", IP: &ip, UserAgent: &userAgent})
	if !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("err=%v", err)
	}
	audit, ok := repo.audit("mfa_code_rejected")
	if !ok || audit.IP == nil || *audit.IP != ip || audit.UserAgent == nil || *audit.UserAgent != userAgent {
		t.Fatalf("mfa_code_rejected audit=%+v found=%t", audit, ok)
	}
	if _, exhausted := repo.audit("mfa_challenge_exhausted"); exhausted {
		t.Fatal("challenge with attempts left was reported exhausted")
	}
}

func TestRF014_CincoCodigosIncorrectosAgotanElDesafio(t *testing.T) {
	repo := &memoryRepository{challenge: activeChallenge("123456", 1)}
	_, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "999999"})
	if !errors.Is(err, ErrCodeInvalid) || repo.challenge.AttemptsLeft != 0 || !repo.challenge.Used {
		t.Fatalf("err=%v challenge=%+v", err, repo.challenge)
	}
	// The exhausting guess is itself a rejected code (it counts toward RF-017);
	// the exhaustion is an additional, non-counted event.
	if repo.count("mfa_code_rejected") != 1 || repo.count("mfa_challenge_exhausted") != 1 {
		t.Fatalf("audits=%+v", repo.audits)
	}
}

func TestRF017_RechazosDeCodigoMFABloqueanLaCuentaAlUmbral(t *testing.T) {
	ip := netip.MustParseAddr("198.51.100.4")
	userAgent := "attacker/1.0"
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts), priorFailures: 1}
	publisher := &fakePublisher{}
	service := newVerifier(t, repo, publisher).WithLockout(lockout.Config{AccountMaxFailures: 3, IPMaxFailures: 20, FailureWindow: time.Hour, LockoutDuration: 30 * time.Minute})
	input := VerifyInput{Token: encodedToken(), Code: "999999", IP: &ip, UserAgent: &userAgent}

	if _, err := service.Verify(context.Background(), input); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("first wrong code err=%v", err)
	}
	if repo.locked || len(publisher.locked()) != 0 {
		t.Fatal("account locked before reaching the threshold")
	}
	if _, err := service.Verify(context.Background(), input); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("second wrong code err=%v", err)
	}
	if !repo.locked {
		t.Fatal("account was not locked at the threshold")
	}
	if !repo.lockedUntil.After(time.Now().Add(29 * time.Minute)) {
		t.Fatalf("lockedUntil=%v, want ~30 minutes ahead", repo.lockedUntil)
	}
	audit, ok := repo.audit("account_locked")
	if !ok || audit.IP == nil || *audit.IP != ip || audit.UserAgent == nil {
		t.Fatalf("account_locked audit=%+v found=%t", audit, ok)
	}
	locked := publisher.locked()
	if len(locked) != 1 || locked[0].Data.FailedAttempts != 3 || locked[0].Data.Email != "ada@example.test" || locked[0].Data.IP != ip.String() || !locked[0].Data.LockedUntil.Equal(repo.lockedUntil) {
		t.Fatalf("security.account_locked events=%+v", locked)
	}
}

func TestRF017_LosDesafiosAbiertosDeUnaCuentaBloqueadaDejanDeVerificar(t *testing.T) {
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts)}
	repo.challenge.User.Status = "locked"
	_, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "123456"})
	if !errors.Is(err, ErrChallengeInvalid) || repo.consumed {
		t.Fatalf("err=%v consumed=%t", err, repo.consumed)
	}
}

func TestRF014_DesafioDeCuentaNoActivaNoEmiteSesion(t *testing.T) {
	challenge := activeChallenge("123456", MaxAttempts)
	challenge.User.Status = "disabled"
	repo := &memoryRepository{challenge: challenge}
	_, err := newVerifier(t, repo, &fakePublisher{}).Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "123456"})
	if err == nil || repo.consumed {
		t.Fatalf("err=%v consumed=%t", err, repo.consumed)
	}
}

func TestRF014_FalloDeAlmacenamientoNoSeConfundeConDesafioInvalido(t *testing.T) {
	storageErr := errors.New("connection reset")
	repo := &memoryRepository{challenge: activeChallenge("123456", MaxAttempts), getErr: storageErr}
	service := newVerifier(t, repo, &fakePublisher{})
	_, verifyErr := service.Verify(context.Background(), VerifyInput{Token: encodedToken(), Code: "123456"})
	resendErr := service.Resend(context.Background(), encodedToken())
	for name, err := range map[string]error{"verify": verifyErr, "resend": resendErr} {
		if !errors.Is(err, storageErr) || errors.Is(err, ErrChallengeInvalid) {
			t.Fatalf("%s err=%v, want wrapped storage error, not ErrChallengeInvalid", name, err)
		}
	}
}

type memoryRepository struct {
	challenge        StoredChallenge
	created          CreateParams
	resentHash       []byte
	consumed         bool
	refresh          RefreshToken
	audits           []AuditEvent
	priorFailures    int64
	locked           bool
	lockedUntil      time.Time
	lastLoginUpdated bool
	getErr           error
}

func (r *memoryRepository) WithinTransaction(_ context.Context, fn func(Writer) error) error {
	return fn(r)
}
func (r *memoryRepository) CreateChallenge(_ context.Context, p CreateParams) error {
	r.created = p
	return nil
}
func (r *memoryRepository) GetChallengeForUpdate(context.Context, []byte) (StoredChallenge, error) {
	if r.getErr != nil {
		return StoredChallenge{}, r.getErr
	}
	if r.challenge.ID == uuid.Nil {
		return StoredChallenge{}, ErrChallengeInvalid
	}
	return r.challenge, nil
}
func (r *memoryRepository) ConsumeChallenge(context.Context, uuid.UUID) error {
	r.consumed = true
	r.challenge.Used = true
	return nil
}
func (r *memoryRepository) RejectChallenge(context.Context, uuid.UUID) (int, error) {
	r.challenge.AttemptsLeft--
	if r.challenge.AttemptsLeft == 0 {
		r.challenge.Used = true
	}
	return r.challenge.AttemptsLeft, nil
}
func (r *memoryRepository) ResendChallenge(_ context.Context, _ uuid.UUID, codeHash []byte) error {
	r.resentHash = codeHash
	return nil
}
func (r *memoryRepository) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	return []string{"user"}, nil
}
func (r *memoryRepository) CreateRefreshToken(_ context.Context, refresh RefreshToken) error {
	r.refresh = refresh
	return nil
}
func (r *memoryRepository) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}
func (r *memoryRepository) CountLoginFailuresByAccount(context.Context, uuid.UUID, time.Time) (int64, error) {
	return r.priorFailures + int64(r.count("mfa_code_rejected")), nil
}
func (r *memoryRepository) LockLoginUser(_ context.Context, _ uuid.UUID, until time.Time) error {
	r.locked, r.lockedUntil = true, until
	r.challenge.User.Status = "locked"
	return nil
}
func (r *memoryRepository) UpdateLastLogin(context.Context, uuid.UUID) error {
	r.lastLoginUpdated = true
	return nil
}

func (r *memoryRepository) count(action string) int {
	n := 0
	for _, audit := range r.audits {
		if audit.Action == action {
			n++
		}
	}
	return n
}

func (r *memoryRepository) audit(action string) (AuditEvent, bool) {
	for _, audit := range r.audits {
		if audit.Action == action {
			return audit, true
		}
	}
	return AuditEvent{}, false
}

type fakePublisher struct{ published []any }

func (p *fakePublisher) Publish(_ context.Context, _ string, payload any) error {
	p.published = append(p.published, payload)
	return nil
}

func (p *fakePublisher) lastMFA(t *testing.T) events.MfaChallengeIssued {
	t.Helper()
	for i := len(p.published) - 1; i >= 0; i-- {
		if event, ok := p.published[i].(events.MfaChallengeIssued); ok {
			return event
		}
	}
	t.Fatal("no mfa challenge event was published")
	return events.MfaChallengeIssued{}
}

func (p *fakePublisher) locked() []events.AccountLocked {
	var out []events.AccountLocked
	for _, payload := range p.published {
		if event, ok := payload.(events.AccountLocked); ok {
			out = append(out, event)
		}
	}
	return out
}
