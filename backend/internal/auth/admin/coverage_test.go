package admin

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

type adminCoverageRepository struct {
	listInput ListInput
	listUsers []User
	listErr   error
	getUser   User
	getErr    error
	writer    Writer
	txErr     error
	txCalls   int
}

func (r *adminCoverageRepository) ListUsers(_ context.Context, input ListInput) ([]User, error) {
	r.listInput = input
	return r.listUsers, r.listErr
}
func (r *adminCoverageRepository) GetUser(context.Context, uuid.UUID) (User, error) {
	return r.getUser, r.getErr
}
func (r *adminCoverageRepository) WithinUserManagementTransaction(_ context.Context, fn func(Writer) error) error {
	r.txCalls++
	if r.txErr != nil {
		return r.txErr
	}
	return fn(r.writer)
}

func TestRNF005_AdminListAndGetValidateInputsAndWrapRepositoryFailures(t *testing.T) {
	status := StatusActive
	repository := &adminCoverageRepository{listUsers: []User{{ID: uuid.New()}}, getUser: User{ID: uuid.New()}}
	service := New(repository)

	users, err := service.ListUsers(context.Background(), ListInput{Status: &status, Limit: 200})
	if err != nil || !reflect.DeepEqual(users, repository.listUsers) || repository.listInput.Limit != 101 {
		t.Fatalf("ListUsers() users=%#v err=%v limit=%d", users, err, repository.listInput.Limit)
	}
	if _, err := service.ListUsers(context.Background(), ListInput{Status: statusPointer("unknown")}); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("ListUsers() invalid status error=%v", err)
	}

	got, err := service.GetUser(context.Background(), repository.getUser.ID)
	if err != nil || got.ID != repository.getUser.ID {
		t.Fatalf("GetUser() user=%#v err=%v", got, err)
	}
	repository.getErr = errors.New("lookup failed")
	if _, err := service.GetUser(context.Background(), uuid.New()); !errors.Is(err, repository.getErr) {
		t.Fatalf("GetUser() error=%v, want wrapped repository error", err)
	}
	if _, err := New(nil).GetUser(context.Background(), uuid.New()); err == nil {
		t.Fatal("GetUser() error=nil for unavailable service")
	}
}

func TestRNF005_AdminUpdateRejectsInvalidAndSelfTargetedChanges(t *testing.T) {
	actorID := uuid.New()
	repository := &adminCoverageRepository{}
	service := New(repository)
	for _, tt := range []struct {
		name  string
		input UpdateInput
		want  error
	}{
		{name: "unknown status", input: UpdateInput{Status: statusPointer("unknown")}, want: ErrInvalidStatus},
		{name: "unknown role", input: UpdateInput{Roles: rolesPointer([]string{"operator"})}, want: ErrInvalidRole},
		{name: "self disable", input: UpdateInput{ActorUserID: actorID, UserID: actorID, Status: statusPointer(StatusDisabled)}, want: ErrSelfDisable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.UpdateUser(context.Background(), tt.input); !errors.Is(err, tt.want) {
				t.Fatalf("UpdateUser() error=%v, want %v", err, tt.want)
			}
			if repository.txCalls != 0 {
				t.Fatalf("transactions=%d, want no repository interaction", repository.txCalls)
			}
		})
	}
	if _, err := New(nil).UpdateUser(context.Background(), UpdateInput{}); err == nil {
		t.Fatal("UpdateUser() error=nil for unavailable service")
	}
}

func TestRNF005_AdminUpdateDeduplicatesRolesAndAuditsMeaningfulChanges(t *testing.T) {
	actorID, targetID := uuid.New(), uuid.New()
	repository := &repositoryStub{
		users: map[uuid.UUID]User{
			actorID:  {ID: actorID, Status: StatusActive},
			targetID: {ID: targetID, Status: StatusActive, Roles: []string{"user"}},
		},
		roles: map[uuid.UUID][]string{actorID: {"admin"}, targetID: {"user"}},
	}
	roles := []string{"admin", "admin", "user"}
	updated, err := New(repository).UpdateUser(context.Background(), UpdateInput{ActorUserID: actorID, UserID: targetID, Roles: &roles})
	if err != nil {
		t.Fatalf("UpdateUser() error=%v", err)
	}
	if want := []string{"admin", "user"}; !reflect.DeepEqual(updated.Roles, want) || !reflect.DeepEqual(repository.roles[targetID], want) {
		t.Fatalf("roles result=%v repository=%v want=%v", updated.Roles, repository.roles[targetID], want)
	}
	if len(repository.audits) != 1 || repository.audits[0].Action != "role_changed" || repository.audits[0].ResourceID != targetID.String() {
		t.Fatalf("audits=%#v", repository.audits)
	}
}

func TestRNF005_AdminUpdateKeepsLastAdminWhenRolesAreChanged(t *testing.T) {
	actorID, targetID := uuid.New(), uuid.New()
	repository := &repositoryStub{
		users: map[uuid.UUID]User{
			targetID: {ID: targetID, Status: StatusActive},
		},
		roles: map[uuid.UUID][]string{targetID: {"admin", "user"}},
	}
	roles := []string{"user"}
	_, err := New(repository).UpdateUser(context.Background(), UpdateInput{ActorUserID: actorID, UserID: targetID, Roles: &roles})
	if !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("UpdateUser() error=%v, want %v", err, ErrLastActiveAdmin)
	}
}

func statusPointer(value Status) *Status    { return &value }
func rolesPointer(value []string) *[]string { return &value }
