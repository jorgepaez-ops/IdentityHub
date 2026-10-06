package rolegrid

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/uuid"
)

var (
	errBoom     = errors.New("boom")
	testActor   = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	testApp     = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	testRoleID  = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	testAddress = netip.MustParseAddr("203.0.113.9")
)

func fixture() (*Service, *fakeWriter) {
	writer := &fakeWriter{
		application: Application{ID: testApp, ClientID: "contabilidad"},
		validKeys:   []string{"reportes.ver", "movimientos.ver_todos", "cierre.ejecutar"},
		role:        Role{ID: testRoleID, ApplicationID: testApp, Name: "contabilidad.auditor", PermissionKeys: []string{"reportes.ver"}},
	}
	return New(&fakeRepository{writer: writer}), writer
}

func createInput(name string, keys ...string) CreateInput {
	return CreateInput{ActorUserID: testActor, ApplicationID: testApp, Name: name, PermissionKeys: keys, IP: testAddress, UserAgent: "tests"}
}

func TestRF021_CreaRolConPermisosYLoAudita(t *testing.T) {
	service, writer := fixture()

	role, err := service.Create(context.Background(), CreateInput{ActorUserID: testActor, ApplicationID: testApp, Name: "contabilidad.auditor", Description: "Solo lectura", PermissionKeys: []string{"reportes.ver", "movimientos.ver_todos", "reportes.ver"}, IP: testAddress, UserAgent: "tests"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if role.Name != "contabilidad.auditor" || strings.Join(role.PermissionKeys, ",") != "movimientos.ver_todos,reportes.ver" {
		t.Fatalf("Create() = %+v, want unique sorted keys", role)
	}
	if writer.createdDescription != "Solo lectura" || strings.Join(writer.replaced, ",") != "movimientos.ver_todos,reportes.ver" {
		t.Fatalf("stored description=%q keys=%v", writer.createdDescription, writer.replaced)
	}
	if len(writer.audits) != 1 {
		t.Fatalf("audits = %d, want 1", len(writer.audits))
	}
	event := writer.audits[0]
	if event.Action != ActionRoleCreated || event.ActorUserID != testActor || event.IP != testAddress || event.UserAgent != "tests" || event.RoleName != "contabilidad.auditor" || len(event.PermissionKeys) != 2 {
		t.Fatalf("audit event = %+v", event)
	}
}

func TestRF021_CreaRechazaNombresInvalidos(t *testing.T) {
	for _, name := range []string{"admin", "user", "auditor", "otra.auditor", "contabilidad.", "contabilidad.a", "contabilidad.Auditor", "contabilidad.-x", "contabilidad.a b", "contabilidad." + strings.Repeat("a", 42)} {
		service, writer := fixture()
		_, err := service.Create(context.Background(), createInput(name, "reportes.ver"))
		if !errors.Is(err, ErrInvalidRoleName) {
			t.Fatalf("Create(%q) error = %v, want ErrInvalidRoleName", name, err)
		}
		if writer.created || len(writer.audits) != 0 {
			t.Fatalf("Create(%q) wrote data despite the invalid name", name)
		}
	}
}

func TestRF021_CreaRechazaPermisoDesconocidoODeOtraAplicacion(t *testing.T) {
	service, writer := fixture()

	_, err := service.Create(context.Background(), createInput("contabilidad.auditor", "reportes.ver", "usuarios.borrar"))
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("Create() error = %v, want ErrInvalidPermission", err)
	}
	if writer.created || len(writer.audits) != 0 {
		t.Fatal("Create() wrote data despite the unknown permission")
	}
}

func TestRF021_CreaRechazaDescripcionLarga(t *testing.T) {
	service, _ := fixture()
	input := createInput("contabilidad.auditor", "reportes.ver")
	input.Description = strings.Repeat("x", MaxDescriptionLength+1)
	if _, err := service.Create(context.Background(), input); !errors.Is(err, ErrInvalidDescription) {
		t.Fatalf("Create() error = %v, want ErrInvalidDescription", err)
	}
}

func TestRF021_CreaPropagaDuplicadoYAplicacionInexistente(t *testing.T) {
	service, writer := fixture()
	writer.createErr = ErrDuplicateRole
	if _, err := service.Create(context.Background(), createInput("contabilidad.auditor", "reportes.ver")); !errors.Is(err, ErrDuplicateRole) {
		t.Fatalf("Create() error = %v, want ErrDuplicateRole", err)
	}
	if len(writer.audits) != 0 {
		t.Fatal("a duplicate role must not be audited")
	}

	service, writer = fixture()
	writer.applicationErr = ErrApplicationNotFound
	if _, err := service.Create(context.Background(), createInput("contabilidad.auditor", "reportes.ver")); !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("Create() error = %v, want ErrApplicationNotFound", err)
	}
}

func TestRF021_ActualizaPermisosYDescripcionYLoAudita(t *testing.T) {
	service, writer := fixture()
	description := "Nueva"
	keys := []string{"cierre.ejecutar", "reportes.ver"}

	role, err := service.Update(context.Background(), UpdateInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID, Description: &description, PermissionKeys: &keys, IP: testAddress, UserAgent: "tests"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if role.Description != "Nueva" || strings.Join(role.PermissionKeys, ",") != "cierre.ejecutar,reportes.ver" {
		t.Fatalf("Update() = %+v", role)
	}
	if writer.updatedDescription != "Nueva" || strings.Join(writer.replaced, ",") != "cierre.ejecutar,reportes.ver" {
		t.Fatalf("stored description=%q keys=%v", writer.updatedDescription, writer.replaced)
	}
	if len(writer.audits) != 1 || writer.audits[0].Action != ActionRoleUpdated || writer.audits[0].RoleName != "contabilidad.auditor" || len(writer.audits[0].PermissionKeys) != 2 {
		t.Fatalf("audits = %+v", writer.audits)
	}
}

func TestRF021_RechazaCambiarPermisosDeRolPropio(t *testing.T) {
	service, writer := fixture()
	writer.actorHolds = true
	keys := []string{"reportes.ver", "cierre.ejecutar"}

	_, err := service.Update(context.Background(), UpdateInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID, PermissionKeys: &keys})
	if !errors.Is(err, ErrSelfPermissionChange) {
		t.Fatalf("Update() error = %v, want ErrSelfPermissionChange", err)
	}
	if writer.replaced != nil || len(writer.audits) != 0 {
		t.Fatal("Update() changed a role the actor holds")
	}
}

func TestRF021_PermiteCambiarDescripcionDeRolPropio(t *testing.T) {
	service, writer := fixture()
	writer.actorHolds = true
	description := "Solo texto"

	if _, err := service.Update(context.Background(), UpdateInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID, Description: &description}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if writer.updatedDescription != "Solo texto" {
		t.Fatalf("description = %q", writer.updatedDescription)
	}
}

func TestRF021_ActualizaRechazaCasosInvalidos(t *testing.T) {
	description := "ok"
	long := strings.Repeat("x", MaxDescriptionLength+1)
	unknown := []string{"nope"}
	tests := []struct {
		name   string
		mutate func(*fakeWriter)
		input  UpdateInput
		want   error
	}{
		{"rol del sistema", func(w *fakeWriter) { w.role.System = true }, UpdateInput{Description: &description}, ErrSystemRole},
		{"rol inexistente", func(w *fakeWriter) { w.roleErr = ErrRoleNotFound }, UpdateInput{Description: &description}, ErrRoleNotFound},
		{"permiso desconocido", func(*fakeWriter) {}, UpdateInput{PermissionKeys: &unknown}, ErrInvalidPermission},
		{"texto largo", func(*fakeWriter) {}, UpdateInput{Description: &long}, ErrInvalidDescription},
		{"sin cambios", func(*fakeWriter) {}, UpdateInput{}, ErrEmptyUpdate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, writer := fixture()
			tt.mutate(writer)
			tt.input.ActorUserID, tt.input.ApplicationID, tt.input.RoleID = testActor, testApp, testRoleID
			if _, err := service.Update(context.Background(), tt.input); !errors.Is(err, tt.want) {
				t.Fatalf("Update() error = %v, want %v", err, tt.want)
			}
			if writer.replaced != nil || writer.updatedDescription != "" || len(writer.audits) != 0 {
				t.Fatal("Update() wrote data despite the error")
			}
		})
	}
}

func TestRF021_EliminaRolSinAsignacionesYLoAudita(t *testing.T) {
	service, writer := fixture()

	err := service.Delete(context.Background(), DeleteInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID, IP: testAddress, UserAgent: "tests"})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !writer.deleted || len(writer.audits) != 1 || writer.audits[0].Action != ActionRoleDeleted || writer.audits[0].RoleName != "contabilidad.auditor" {
		t.Fatalf("deleted=%t audits=%+v", writer.deleted, writer.audits)
	}
}

func TestRF021_EliminaRechazaRolAsignadoPropioODelSistema(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeWriter)
		want   error
	}{
		{"asignado", func(w *fakeWriter) { w.role.AssignedCount = 2 }, ErrRoleAssigned},
		{"propio y asignado: gana el control 2", func(w *fakeWriter) { w.actorHolds = true; w.role.AssignedCount = 1 }, ErrSelfRoleDelete},
		{"propio", func(w *fakeWriter) { w.actorHolds = true }, ErrSelfRoleDelete},
		{"sistema", func(w *fakeWriter) { w.role.System = true }, ErrSystemRole},
		{"inexistente", func(w *fakeWriter) { w.roleErr = ErrRoleNotFound }, ErrRoleNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, writer := fixture()
			tt.mutate(writer)
			err := service.Delete(context.Background(), DeleteInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID})
			if !errors.Is(err, tt.want) {
				t.Fatalf("Delete() error = %v, want %v", err, tt.want)
			}
			if writer.deleted || len(writer.audits) != 0 {
				t.Fatal("Delete() removed the role despite the error")
			}
		})
	}
}

func TestRF021_UnFalloDeAuditoriaAbortaLaTransaccion(t *testing.T) {
	keys := []string{"reportes.ver"}
	description := "x"
	operations := map[string]func(*Service) error{
		"create": func(s *Service) error {
			_, err := s.Create(context.Background(), createInput("contabilidad.auditor", "reportes.ver"))
			return err
		},
		"update": func(s *Service) error {
			_, err := s.Update(context.Background(), UpdateInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID, PermissionKeys: &keys, Description: &description})
			return err
		},
		"delete": func(s *Service) error {
			return s.Delete(context.Background(), DeleteInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID})
		},
	}
	for name, run := range operations {
		t.Run(name, func(t *testing.T) {
			service, writer := fixture()
			writer.auditErr = errBoom
			repository := service.repository.(*fakeRepository)
			if err := run(service); !errors.Is(err, errBoom) {
				t.Fatalf("error = %v, want the audit failure", err)
			}
			if repository.committed {
				t.Fatal("the transaction must not commit when the audit write fails")
			}
		})
	}
}

func TestRF021_ListaYReportaServicioNoDisponible(t *testing.T) {
	repository := &fakeRepository{writer: &fakeWriter{}, applications: []Application{{ClientID: "contabilidad"}}, roles: []Role{{Name: "contabilidad.auditor"}}}
	service := New(repository)
	if apps, err := service.ListApplications(context.Background()); err != nil || len(apps) != 1 {
		t.Fatalf("ListApplications() = %v, %v", apps, err)
	}
	if roles, err := service.ListRoles(context.Background(), testApp); err != nil || len(roles) != 1 {
		t.Fatalf("ListRoles() = %v, %v", roles, err)
	}

	unavailable := New(nil)
	if _, err := unavailable.ListApplications(context.Background()); err == nil {
		t.Fatal("ListApplications() without repository succeeded")
	}
	if _, err := unavailable.ListRoles(context.Background(), testApp); err == nil {
		t.Fatal("ListRoles() without repository succeeded")
	}
	if _, err := unavailable.Create(context.Background(), CreateInput{}); err == nil {
		t.Fatal("Create() without repository succeeded")
	}
	if _, err := unavailable.Update(context.Background(), UpdateInput{}); err == nil {
		t.Fatal("Update() without repository succeeded")
	}
	if err := unavailable.Delete(context.Background(), DeleteInput{}); err == nil {
		t.Fatal("Delete() without repository succeeded")
	}
}

func TestRF021_PropagaErroresDeLasConsultasAuxiliares(t *testing.T) {
	keys := []string{"reportes.ver"}
	steps := map[string]func(*fakeWriter){
		"validar permisos": func(w *fakeWriter) { w.validateErr = errBoom },
		"rol del actor":    func(w *fakeWriter) { w.holdsErr = errBoom },
		"reemplazar":       func(w *fakeWriter) { w.replaceErr = errBoom },
	}
	for name, mutate := range steps {
		t.Run(name, func(t *testing.T) {
			service, writer := fixture()
			mutate(writer)
			_, err := service.Update(context.Background(), UpdateInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID, PermissionKeys: &keys})
			if !errors.Is(err, errBoom) {
				t.Fatalf("Update() error = %v, want %v", err, errBoom)
			}
		})
	}
	service, writer := fixture()
	writer.holdsErr = errBoom
	if err := service.Delete(context.Background(), DeleteInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID}); !errors.Is(err, errBoom) {
		t.Fatalf("Delete() error = %v, want %v", err, errBoom)
	}
}

type fakeRepository struct {
	writer       *fakeWriter
	applications []Application
	roles        []Role
	committed    bool
}

func (r *fakeRepository) ListApplications(context.Context) ([]Application, error) {
	return r.applications, nil
}
func (r *fakeRepository) ListRoles(context.Context, uuid.UUID) ([]Role, error) { return r.roles, nil }
func (r *fakeRepository) WithinRoleGridTransaction(_ context.Context, fn func(Writer) error) error {
	if err := fn(r.writer); err != nil {
		return err
	}
	r.committed = true
	return nil
}

type fakeWriter struct {
	application        Application
	applicationErr     error
	role               Role
	roleErr            error
	validKeys          []string
	validateErr        error
	actorHolds         bool
	holdsErr           error
	createErr          error
	replaceErr         error
	auditErr           error
	created            bool
	createdDescription string
	replaced           []string
	updatedDescription string
	deleted            bool
	audits             []AuditEvent
	lockedReads        int
}

func (w *fakeWriter) GetApplication(context.Context, uuid.UUID) (Application, error) {
	return w.application, w.applicationErr
}
func (w *fakeWriter) GetApplicationRoleForUpdate(context.Context, uuid.UUID, uuid.UUID) (Role, error) {
	w.lockedReads++
	return w.role, w.roleErr
}
func (w *fakeWriter) ValidatePermissionKeys(_ context.Context, _ uuid.UUID, keys []string) ([]string, error) {
	var found []string
	for _, key := range keys {
		for _, valid := range w.validKeys {
			if key == valid {
				found = append(found, key)
			}
		}
	}
	return found, w.validateErr
}
func (w *fakeWriter) ActorHoldsApplicationRole(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return w.actorHolds, w.holdsErr
}
func (w *fakeWriter) CreateApplicationRole(_ context.Context, applicationID uuid.UUID, name, description string) (Role, error) {
	if w.createErr != nil {
		return Role{}, w.createErr
	}
	w.created, w.createdDescription = true, description
	return Role{ID: testRoleID, ApplicationID: applicationID, Name: name, Description: description}, nil
}
func (w *fakeWriter) ReplaceApplicationRolePermissions(_ context.Context, _, _ uuid.UUID, keys []string) error {
	w.replaced = append([]string(nil), keys...)
	return w.replaceErr
}
func (w *fakeWriter) UpdateApplicationRoleDescription(_ context.Context, _ uuid.UUID, description string) error {
	w.updatedDescription = description
	return nil
}
func (w *fakeWriter) DeleteApplicationRole(context.Context, uuid.UUID) error {
	w.deleted = true
	return nil
}
func (w *fakeWriter) InsertRoleGridAuditEvent(_ context.Context, event AuditEvent) error {
	if w.auditErr != nil {
		return w.auditErr
	}
	w.audits = append(w.audits, event)
	return nil
}

func TestRF021_ActualizaYEliminaLeenElRolConBloqueoDeFila(t *testing.T) {
	description := "Nueva"
	service, writer := fixture()
	if _, err := service.Update(context.Background(), UpdateInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID, Description: &description}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if writer.lockedReads != 1 {
		t.Fatalf("Update() locked reads = %d, want 1 so controls 2 and 4 see a locked role row", writer.lockedReads)
	}
	service, writer = fixture()
	if err := service.Delete(context.Background(), DeleteInput{ActorUserID: testActor, ApplicationID: testApp, RoleID: testRoleID}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if writer.lockedReads != 1 {
		t.Fatalf("Delete() locked reads = %d, want 1", writer.lockedReads)
	}
}
