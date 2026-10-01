package roles_test

import (
	"reflect"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/auth/roles"
)

func TestRF009_CatalogoAceptaRolesDeDirectorioYDeNegocio(t *testing.T) {
	for _, role := range []string{roles.Admin, roles.User, roles.ContabilidadSenior, roles.ContabilidadAnalista} {
		if !roles.Valid(role) {
			t.Errorf("Valid(%q) = false, want true", role)
		}
	}
}

func TestRF009_CatalogoRechazaUnRolDesconocido(t *testing.T) {
	if roles.Valid("operator") {
		t.Fatal("Valid(\"operator\") = true, want false: not in the D8 catalog")
	}
	if roles.Valid("") {
		t.Fatal(`Valid("") = true, want false`)
	}
}

func TestRF009_DirectoryDevuelveSoloRolesDeDirectorio(t *testing.T) {
	all := []string{roles.Admin, roles.ContabilidadSenior, roles.User, roles.ContabilidadAnalista}
	got := roles.Directory(all)
	want := []string{roles.Admin, roles.User}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Directory(%v) = %v, want %v", all, got, want)
	}
}

func TestRF009_DirectoryDeUnEmpleadoSoloConUserNoContieneRolesDeAplicacion(t *testing.T) {
	got := roles.Directory([]string{roles.User})
	if !reflect.DeepEqual(got, []string{roles.User}) {
		t.Fatalf("Directory([user]) = %v, want [user]", got)
	}
}

func TestRF009_ForApplicationFiltraPorPrefijo(t *testing.T) {
	all := []string{roles.Admin, roles.User, roles.ContabilidadSenior, roles.ContabilidadAnalista}
	got := roles.ForApplication(all, roles.ApplicationContabilidad)
	want := []string{roles.ContabilidadSenior, roles.ContabilidadAnalista}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ForApplication(%v, %q) = %v, want %v", all, roles.ApplicationContabilidad, got, want)
	}
}

func TestRF009_ForApplicationVacioEquivaleADirectory(t *testing.T) {
	all := []string{roles.Admin, roles.ContabilidadSenior}
	if got, want := roles.ForApplication(all, ""), roles.Directory(all); !reflect.DeepEqual(got, want) {
		t.Fatalf("ForApplication(all, \"\") = %v, want %v (same as Directory)", got, want)
	}
}

func TestRF009_DirectoryDescartaNombresSinPuntoFueraDelCatalogo(t *testing.T) {
	got := roles.Directory([]string{roles.User, "operator", roles.Admin})
	want := []string{roles.User, roles.Admin}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Directory() = %v, want %v: an unknown dotless name is not a directory role", got, want)
	}
}
