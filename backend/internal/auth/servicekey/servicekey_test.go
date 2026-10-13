package servicekey

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	records  []Record
	audits   []AuditEvent
	touched  []uuid.UUID
	auditErr error
	touchErr error
}

func (f *fakeRepository) Create(_ context.Context, params CreateParams, event AuditEvent) (Key, error) {
	key := Key{ID: uuid.New(), PublicID: params.PublicID, Name: params.Name, Scopes: params.Scopes, CreatedBy: &params.CreatedBy, ExpiresAt: params.ExpiresAt, CreatedAt: params.ExpiresAt.Add(-time.Hour)}
	f.records = append(f.records, Record{Key: key, SecretHash: params.SecretHash})
	event.KeyID = &key.ID
	f.audits = append(f.audits, event)
	return key, nil
}
func (f *fakeRepository) List(context.Context) ([]Key, error) {
	keys := make([]Key, 0, len(f.records))
	for _, record := range f.records {
		keys = append(keys, record.Key)
	}
	return keys, nil
}
func (f *fakeRepository) Revoke(_ context.Context, id uuid.UUID, event AuditEvent) error {
	for index, record := range f.records {
		if record.ID != id {
			continue
		}
		if record.RevokedAt == nil {
			now := time.Now()
			f.records[index].RevokedAt = &now
			f.audits = append(f.audits, event)
		}
		return nil
	}
	return ErrNotFound
}
func (f *fakeRepository) GetByPublicID(_ context.Context, publicID string) (Record, error) {
	for _, record := range f.records {
		if record.PublicID == publicID {
			return record, nil
		}
	}
	return Record{}, ErrNotFound
}
func (f *fakeRepository) TouchLastUsed(_ context.Context, id uuid.UUID, _ time.Time) error {
	f.touched = append(f.touched, id)
	return f.touchErr
}
func (f *fakeRepository) RecordAudit(_ context.Context, event AuditEvent) error {
	if f.auditErr != nil {
		return f.auditErr
	}
	f.audits = append(f.audits, event)
	return nil
}

var fixedNow = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

func newService() (*Service, *fakeRepository) {
	repository := &fakeRepository{}
	service := New(repository)
	service.now = func() time.Time { return fixedNow }
	return service, repository
}

func validInput() CreateInput {
	return CreateInput{ActorUserID: uuid.New(), Name: "SIEM", Scopes: []string{ScopeAuditLogRead}, ExpiresInDays: 30, IP: netip.MustParseAddr("203.0.113.9"), UserAgent: "tests"}
}

func TestRF018_CreateReturnsFullKeyOnceAndStoresOnlyTheHash(t *testing.T) {
	service, repository := newService()
	created, err := service.Create(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	publicID, secret, ok := parse(created.FullKey)
	if !ok || !strings.HasPrefix(created.FullKey, "ihk_"+created.PublicID+"_") || publicID != created.PublicID {
		t.Fatalf("key %q does not follow ihk_<publicId>_<secret>", created.FullKey)
	}
	if len(secret) != 43 {
		t.Fatalf("secret length=%d, want 43 (32 bytes base64url)", len(secret))
	}
	record := repository.records[0]
	if want := sha256.Sum256([]byte(secret)); string(record.SecretHash) != string(want[:]) {
		t.Fatalf("stored hash is not SHA-256 of the secret")
	}
	if strings.Contains(created.Prefix(), secret) || created.Prefix() != "ihk_"+created.PublicID {
		t.Fatalf("prefix=%q must be public only", created.Prefix())
	}
	if !created.ExpiresAt.Equal(fixedNow.Add(30 * 24 * time.Hour)) {
		t.Fatalf("expiresAt=%v", created.ExpiresAt)
	}
	listed, _ := service.List(context.Background())
	if len(listed) != 1 || listed[0].PublicID != created.PublicID {
		t.Fatalf("list=%+v", listed)
	}
}

func TestRF018_CreateRecordsAnAuditEventWithoutTheSecret(t *testing.T) {
	service, repository := newService()
	input := validInput()
	created, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.audits) != 1 {
		t.Fatalf("audits=%d, want 1", len(repository.audits))
	}
	event := repository.audits[0]
	_, secret, _ := parse(created.FullKey)
	if event.Action != ActionCreated || event.PublicID != created.PublicID || *event.ActorUserID != input.ActorUserID || event.IP != input.IP {
		t.Fatalf("event=%+v", event)
	}
	for _, value := range event.Metadata {
		if strings.Contains(strings.ToLower(toString(value)), secret) {
			t.Fatalf("audit metadata leaks the secret: %+v", event.Metadata)
		}
	}
}

func toString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func TestRF018_CreateRejectsInvalidInput(t *testing.T) {
	cases := map[string]func(*CreateInput){
		"empty name":       func(i *CreateInput) { i.Name = "  " },
		"long name":        func(i *CreateInput) { i.Name = strings.Repeat("a", 101) },
		"no scopes":        func(i *CreateInput) { i.Scopes = nil },
		"unknown scope":    func(i *CreateInput) { i.Scopes = []string{"users:write"} },
		"zero days":        func(i *CreateInput) { i.ExpiresInDays = 0 },
		"negative days":    func(i *CreateInput) { i.ExpiresInDays = -1 },
		"more than a year": func(i *CreateInput) { i.ExpiresInDays = 366 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			service, repository := newService()
			input := validInput()
			mutate(&input)
			if _, err := service.Create(context.Background(), input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err=%v, want ErrInvalidInput", err)
			}
			if len(repository.records) != 0 {
				t.Fatal("an invalid request must not store a key")
			}
		})
	}
	service, _ := newService()
	input := validInput()
	input.Scopes = []string{ScopeAuditLogRead, ScopeAuditLogRead}
	created, err := service.Create(context.Background(), input)
	if err != nil || len(created.Scopes) != 1 {
		t.Fatalf("duplicate scopes must collapse: %v %+v", err, created.Scopes)
	}
}

func TestRF018_AuthenticateAcceptsAValidKeyWithTheScope(t *testing.T) {
	service, repository := newService()
	created, _ := service.Create(context.Background(), validInput())

	principal, err := service.Authenticate(context.Background(), created.FullKey, ScopeAuditLogRead, RequestMeta{})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if principal.PublicID != created.PublicID || principal.KeyID != created.ID {
		t.Fatalf("principal=%+v", principal)
	}
	if len(repository.touched) != 1 {
		t.Fatalf("last_used_at must be updated on use, touched=%d", len(repository.touched))
	}
}

func TestRF018_AuthenticateRejectsEveryFailureWithTheSameError(t *testing.T) {
	service, repository := newService()
	valid, _ := service.Create(context.Background(), validInput())
	_, secret, _ := parse(valid.FullKey)

	expiredInput := validInput()
	expired, _ := service.Create(context.Background(), expiredInput)
	repository.records[1].ExpiresAt = fixedNow.Add(-time.Second)

	revoked, _ := service.Create(context.Background(), validInput())
	if err := service.Revoke(context.Background(), revoked.ID, RevokeInput{ActorUserID: uuid.New()}); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"wrong value": "ihk_" + valid.PublicID + "_" + strings.Repeat("A", 43),
		"unknown id":  "ihk_0000000000000000_" + secret,
		"expired":     expired.FullKey,
		"revoked":     revoked.FullKey,
		"malformed":   "ihk_nonsense",
		"no prefix":   valid.PublicID + "_" + secret,
		"empty value": "ihk_" + valid.PublicID + "_",
		"huge input":  "ihk_" + strings.Repeat("a", 10000),
	}
	before := len(repository.audits)
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := service.Authenticate(context.Background(), raw, ScopeAuditLogRead, RequestMeta{})
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("err=%v, want ErrUnauthenticated", err)
			}
			if err.Error() != ErrUnauthenticated.Error() {
				t.Fatalf("error text differs per cause: %q", err)
			}
		})
	}
	rejected := 0
	for _, event := range repository.audits[before:] {
		if event.Action == ActionRejected {
			rejected++
			if strings.Contains(event.PublicID, secret) {
				t.Fatalf("rejection audit leaks the secret")
			}
		}
	}
	// Only rejections tied to an existing key are audited (wrong value, expired, revoked);
	// unknown ids and malformed input are anonymous junk and must not write to the database.
	if rejected != 3 {
		t.Fatalf("rejected audit events=%d, want 3", rejected)
	}
}

func TestRF018_AuthenticateRefusesAKeyWithoutTheScope(t *testing.T) {
	service, _ := newService()
	created, _ := service.Create(context.Background(), validInput())
	if _, err := service.Authenticate(context.Background(), created.FullKey, "users:write", RequestMeta{}); !errors.Is(err, ErrInsufficientScope) {
		t.Fatalf("err=%v, want ErrInsufficientScope", err)
	}
}

func TestRF018_RevokeIsIdempotentAndAuditedOnce(t *testing.T) {
	service, repository := newService()
	created, _ := service.Create(context.Background(), validInput())
	actor := uuid.New()
	for range 2 {
		if err := service.Revoke(context.Background(), created.ID, RevokeInput{ActorUserID: actor}); err != nil {
			t.Fatalf("Revoke: %v", err)
		}
	}
	revocations := 0
	for _, event := range repository.audits {
		if event.Action == ActionRevoked {
			revocations++
		}
	}
	if revocations != 1 {
		t.Fatalf("revocation audits=%d, want 1", revocations)
	}
	if err := service.Revoke(context.Background(), uuid.New(), RevokeInput{ActorUserID: actor}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id err=%v, want ErrNotFound", err)
	}
}

func loggedService(repository *fakeRepository) (*Service, *bytes.Buffer) {
	buffer := &bytes.Buffer{}
	service := New(repository).WithLogger(slog.New(slog.NewTextHandler(buffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
	service.now = func() time.Time { return fixedNow }
	return service, buffer
}

func TestRF018_AnonymousJunkIsLoggedAndNeverWrittenToTheAuditLog(t *testing.T) {
	repository := &fakeRepository{}
	service, logs := loggedService(repository)
	created, _ := service.Create(context.Background(), validInput())
	_, secret, _ := parse(created.FullKey)
	before := len(repository.audits)
	meta := RequestMeta{IP: netip.MustParseAddr("198.51.100.7")}

	cases := map[string]struct{ raw, reason string }{
		"unknown id": {"ihk_0000000000000000_" + secret, "unknown_key"},
		"malformed":  {"ihk_not-a-key-" + secret, "malformed"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			logs.Reset()
			if _, err := service.Authenticate(context.Background(), tc.raw, ScopeAuditLogRead, meta); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("err=%v, want ErrUnauthenticated", err)
			}
			if len(repository.audits) != before {
				t.Fatalf("anonymous rejection wrote to the audit log: %+v", repository.audits[before:])
			}
			out := logs.String()
			for _, want := range []string{"level=WARN", "reason=" + tc.reason, "scope=" + ScopeAuditLogRead, "ip=198.51.100.7"} {
				if !strings.Contains(out, want) {
					t.Fatalf("log %q lacks %q", out, want)
				}
			}
			if strings.Contains(out, secret) || strings.Contains(out, tc.raw) {
				t.Fatalf("log leaks the secret or the raw header: %q", out)
			}
		})
	}
	logs.Reset()
	_, _ = service.Authenticate(context.Background(), "ihk_0000000000000000_"+secret, ScopeAuditLogRead, meta)
	if !strings.Contains(logs.String(), "public_id=0000000000000000") {
		t.Fatalf("a well-formed public id should be logged: %q", logs.String())
	}
	logs.Reset()
	_, _ = service.Authenticate(context.Background(), "ihk_garbage", ScopeAuditLogRead, meta)
	if strings.Contains(logs.String(), "public_id") || strings.Contains(logs.String(), "garbage") {
		t.Fatalf("a malformed value must not be logged: %q", logs.String())
	}
}

func TestRF018_RejectionsTiedToAnExistingKeyAreAudited(t *testing.T) {
	service, repository := newService()
	valid, _ := service.Create(context.Background(), validInput())
	expired, _ := service.Create(context.Background(), validInput())
	repository.records[1].ExpiresAt = fixedNow.Add(-time.Second)
	revoked, _ := service.Create(context.Background(), validInput())
	_ = service.Revoke(context.Background(), revoked.ID, RevokeInput{ActorUserID: uuid.New()})

	cases := map[string]struct {
		raw, scope, reason string
		want               error
		id                 uuid.UUID
	}{
		"bad_secret":         {"ihk_" + valid.PublicID + "_" + strings.Repeat("A", 43), ScopeAuditLogRead, "bad_secret", ErrUnauthenticated, valid.ID},
		"expired":            {expired.FullKey, ScopeAuditLogRead, "expired", ErrUnauthenticated, expired.ID},
		"revoked":            {revoked.FullKey, ScopeAuditLogRead, "revoked", ErrUnauthenticated, revoked.ID},
		"insufficient_scope": {valid.FullKey, "users:write", "insufficient_scope", ErrInsufficientScope, valid.ID},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			before := len(repository.audits)
			if _, err := service.Authenticate(context.Background(), tc.raw, tc.scope, RequestMeta{}); !errors.Is(err, tc.want) {
				t.Fatalf("err=%v, want %v", err, tc.want)
			}
			if len(repository.audits) != before+1 {
				t.Fatalf("audits=%d, want one new event", len(repository.audits)-before)
			}
			event := repository.audits[before]
			if event.Action != ActionRejected || event.KeyID == nil || *event.KeyID != tc.id || event.Metadata["reason"] != tc.reason {
				t.Fatalf("event=%+v", event)
			}
		})
	}
}

func TestRF018_AuditWriteFailureIsLoggedAndTheRejectionStands(t *testing.T) {
	repository := &fakeRepository{}
	service, logs := loggedService(repository)
	created, _ := service.Create(context.Background(), validInput())
	repository.auditErr = errors.New("database down")

	_, err := service.Authenticate(context.Background(), "ihk_"+created.PublicID+"_"+strings.Repeat("A", 43), ScopeAuditLogRead, RequestMeta{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v, want ErrUnauthenticated", err)
	}
	out := logs.String()
	for _, want := range []string{"level=ERROR", "database down", "key_id=" + created.ID.String(), "action=" + ActionRejected} {
		if !strings.Contains(out, want) {
			t.Fatalf("log %q lacks %q", out, want)
		}
	}
}

func TestRF018_TouchFailureIsLoggedAndAuthenticationSucceeds(t *testing.T) {
	repository := &fakeRepository{}
	service, logs := loggedService(repository)
	created, _ := service.Create(context.Background(), validInput())
	repository.touchErr = errors.New("touch failed")

	principal, err := service.Authenticate(context.Background(), created.FullKey, ScopeAuditLogRead, RequestMeta{})
	if err != nil || principal.KeyID != created.ID {
		t.Fatalf("principal=%+v err=%v, want success", principal, err)
	}
	out := logs.String()
	for _, want := range []string{"level=WARN", "touch failed", "key_id=" + created.ID.String(), "action=touch_last_used"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log %q lacks %q", out, want)
		}
	}
}
