package roles_test

import (
	"reflect"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/auth/roles"
)

func TestRF009_DirectoryDevuelveSoloRolesDeDirectorio(t *testing.T) {
	all := []string{roles.Admin, "contabilidad.senior", roles.User, "contabilidad.analista"}
	got := roles.Directory(all)
	want := []string{roles.Admin, roles.User}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Directory(%v) = %v, want %v", all, got, want)
	}
}

func TestRF009_ForApplicationFiltraPorPrefijo(t *testing.T) {
	all := []string{roles.Admin, roles.User, "contabilidad.senior", "contabilidad.analista", "otra.operador"}
	got := roles.ForApplication(all, "contabilidad")
	want := []string{"contabilidad.senior", "contabilidad.analista"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ForApplication(%v) = %v, want %v", all, got, want)
	}
}

func TestRF009_DirectoryDescartaNombresSinPuntoFueraDelDirectorio(t *testing.T) {
	got := roles.Directory([]string{roles.User, "operator", roles.Admin})
	want := []string{roles.User, roles.Admin}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Directory() = %v, want %v", got, want)
	}
}
