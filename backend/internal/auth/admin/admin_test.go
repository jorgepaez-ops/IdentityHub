package admin

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type repositoryStub struct {
	users  map[uuid.UUID]User
	roles  map[uuid.UUID][]string
	audits []AuditEvent
}

func (r *repositoryStub) ListUsers(context.Context, ListInput) ([]User, error) { return nil, nil }
func (r *repositoryStub) GetUser(_ context.Context, id uuid.UUID) (User, error) {
	u, ok := r.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}
func (r *repositoryStub) WithinUserManagementTransaction(_ context.Context, fn func(Writer) error) error {
	return fn(r)
}
func (r *repositoryStub) LockActiveAdmins(context.Context) (int64, error) {
	var n int64
	for id, u := range r.users {
		if u.Status == StatusActive && testHasRole(r.roles[id], "admin") {
			n++
		}
	}
	return n, nil
}
func (r *repositoryStub) GetUserForUpdate(ctx context.Context, id uuid.UUID) (User, error) {
	return r.GetUser(ctx, id)
}
func (r *repositoryStub) ListRolesForUser(_ context.Context, id uuid.UUID) ([]string, error) {
	return append([]string(nil), r.roles[id]...), nil
}
func (r *repositoryStub) UpdateUser(_ context.Context, id uuid.UUID, status Status) (User, error) {
	u := r.users[id]
	u.Status = status
	r.users[id] = u
	return u, nil
}
func (r *repositoryStub) ReplaceRoles(_ context.Context, id uuid.UUID, roles []string, _ uuid.UUID) error {
	r.roles[id] = append([]string(nil), roles...)
	return nil
}
func (r *repositoryStub) InsertAuditEvent(_ context.Context, event AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

func testHasRole(roles []string, role string) bool {
	for _, value := range roles {
		if value == role {
			return true
		}
	}
	return false
}

func TestRF010_NoSeDejaElSistemaSinAdmin(t *testing.T) {
	adminID := uuid.New()
	repository := &repositoryStub{users: map[uuid.UUID]User{adminID: {ID: adminID, Status: StatusActive}}, roles: map[uuid.UUID][]string{adminID: {"admin"}}}
	_, err := New(repository).UpdateUser(context.Background(), UpdateInput{ActorUserID: uuid.New(), UserID: adminID, Status: statusPtr(StatusDisabled)})
	if !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("UpdateUser error=%v, want ErrLastActiveAdmin", err)
	}
	if repository.users[adminID].Status != StatusActive {
		t.Fatal("the last active admin was disabled")
	}
}

func TestRF010_AdminDeshabilitaYElUsuarioNoEntra(t *testing.T) {
	actorID, targetID := uuid.New(), uuid.New()
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{users: map[uuid.UUID]User{
		actorID: {ID: actorID, Status: StatusActive}, targetID: {ID: targetID, Email: "user@example.com", PasswordHash: hash, Status: StatusActive},
	}, roles: map[uuid.UUID][]string{actorID: {"admin"}, targetID: {"user"}}}
	if _, err := New(repository).UpdateUser(context.Background(), UpdateInput{ActorUserID: actorID, UserID: targetID, Status: statusPtr(StatusDisabled)}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if repository.users[targetID].Status != StatusDisabled {
		t.Fatal("target was not disabled")
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "user_disabled" || repository.audits[0].ActorUserID == nil || *repository.audits[0].ActorUserID != actorID {
		t.Fatalf("audits=%+v", repository.audits)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = login.New(loginRepository{repository}, signer, time.Hour).Login(context.Background(), login.Input{Email: "user@example.com", Password: "correct horse battery"})
	if !errors.Is(err, login.ErrInvalidCredentials) {
		t.Fatalf("disabled user login error=%v", err)
	}
}

type loginRepository struct{ *repositoryStub }

func (r loginRepository) WithinLoginTransaction(ctx context.Context, fn func(login.Writer) error) error {
	return fn(loginWriter(r))
}

type loginWriter struct{ *repositoryStub }

func (w loginWriter) GetLoginUserByEmail(_ context.Context, email string) (login.User, error) {
	for _, u := range w.users {
		if u.Email == email {
			return login.User{ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash, Status: login.Status(u.Status)}, nil
		}
	}
	return login.User{}, errors.New("not found")
}
func (w loginWriter) UpdateLoginSuccess(context.Context, uuid.UUID, string) error  { return nil }
func (w loginWriter) CreateRefreshToken(context.Context, login.RefreshToken) error { return nil }
func (w loginWriter) CountLoginFailuresByAccount(context.Context, uuid.UUID, time.Time) (int64, error) {
	return 0, nil
}
func (w loginWriter) CountLoginFailuresByIP(context.Context, netip.Addr, time.Time) (int64, error) {
	return 0, nil
}
func (w loginWriter) LockLoginUser(context.Context, uuid.UUID, time.Time) error { return nil }
func (w loginWriter) UnlockLoginUser(context.Context, uuid.UUID) error          { return nil }
func (w loginWriter) ListRolesForUser(ctx context.Context, id uuid.UUID) ([]string, error) {
	return w.repositoryStub.ListRolesForUser(ctx, id)
}
func (w loginWriter) InsertAuditEvent(_ context.Context, event login.AuditEvent) error {
	return w.repositoryStub.InsertAuditEvent(context.Background(), AuditEvent{ActorUserID: event.ActorUserID, Action: event.Action})
}

func statusPtr(value Status) *Status { return &value }
