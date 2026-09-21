package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// RNF-003 — Un secreto ausente tiene que impedir el arranque. Una aplicación
// que arranca con una contraseña por defecto es peor que una que no arranca,
// porque nadie se entera de que está expuesta.
func TestRNF003_FaltaDeSecretoImpideElArranque(t *testing.T) {
	casos := []struct {
		nombre  string
		entorno map[string]string
		falta   string
	}{
		{
			nombre:  "sin DATABASE_URL",
			entorno: map[string]string{"RABBITMQ_URL": "amqp://x"},
			falta:   "DATABASE_URL",
		},
		{
			nombre:  "sin RABBITMQ_URL",
			entorno: map[string]string{"DATABASE_URL": "postgres://x"},
			falta:   "RABBITMQ_URL",
		},
		{
			nombre:  "entorno vacío",
			entorno: map[string]string{},
			falta:   "DATABASE_URL",
		},
		{
			nombre: "una variable presente pero en blanco no cuenta como presente",
			entorno: map[string]string{
				"DATABASE_URL": "   ",
				"RABBITMQ_URL": "amqp://x",
			},
			falta: "DATABASE_URL",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			for k, v := range c.entorno {
				t.Setenv(k, v)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("Load() devolvió nil; se esperaba un error por secreto ausente")
			}
			if !strings.Contains(err.Error(), c.falta) {
				t.Errorf("el error debería nombrar %q para que sea accionable; se obtuvo: %v", c.falta, err)
			}
		})
	}
}

// Los errores se acumulan en lugar de abortar en el primero: arrancar el
// contenedor cinco veces para descubrir cinco variables faltantes es una forma
// tonta de perder una tarde.
func TestRNF003_AcumulaTodosLosErroresDeUnaVez(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("RABBITMQ_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("se esperaba un error")
	}

	msg := err.Error()
	for _, esperada := range []string{"DATABASE_URL", "RABBITMQ_URL"} {
		if !strings.Contains(msg, esperada) {
			t.Errorf("el error debería mencionar %s; se obtuvo:\n%s", esperada, msg)
		}
	}
}

// RNF-012 / AM-013 — Un secreto nunca debe poder acabar en los logs por
// descuido. El tipo Secret hace que formatear la estructura que lo contiene sea
// seguro por defecto, en vez de depender de que nadie se equivoque.
func TestRNF012_SecretNoSeRevelaAlFormatearla(t *testing.T) {
	const valor = "postgres://identity:contraseña-real@db:5432/identity"
	s := Secret(valor)

	comprobaciones := map[string]string{
		"%v con String()":  fmt.Sprintf("%v", s),
		"%s con String()":  fmt.Sprint(s),
		"%q":               fmt.Sprintf("%q", s),
		"%#v con GoString": fmt.Sprintf("%#v", s),
		"dentro de struct": fmt.Sprintf("%+v", struct{ URL Secret }{s}),
	}

	for nombre, salida := range comprobaciones {
		if strings.Contains(salida, "contraseña-real") {
			t.Errorf("%s filtró el secreto: %s", nombre, salida)
		}
		if !strings.Contains(salida, "REDACTADO") {
			t.Errorf("%s debería mostrar [REDACTADO]; se obtuvo: %s", nombre, salida)
		}
	}

	// Serializar la configuración a JSON —por ejemplo en un endpoint de
	// diagnóstico— tampoco puede filtrarla.
	crudo, err := json.Marshal(struct {
		URL Secret `json:"url"`
	}{s})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(crudo), "contraseña-real") {
		t.Errorf("la serialización JSON filtró el secreto: %s", crudo)
	}

	// Y aun así el valor sigue siendo utilizable donde hace falta de verdad.
	if s.Reveal() != valor {
		t.Errorf("Reveal() = %q; se esperaba %q", s.Reveal(), valor)
	}
}

func TestRNF003_FaltaClaveDeFirmaImpideElArranque(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("RABBITMQ_URL", "")
	t.Setenv("JWT_SIGNING_KEY", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SIGNING_KEY") || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "RABBITMQ_URL") {
		t.Fatalf("Load did not accumulate mandatory configuration errors: %v", err)
	}

	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("RABBITMQ_URL", "amqp://x")
	t.Setenv("JWT_SIGNING_KEY", "not-base64")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SIGNING_KEY") {
		t.Fatalf("Load malformed JWT_SIGNING_KEY error = %v", err)
	}
}

func TestRNF003_ClaveDeFirmaNoSeRevelaEnConfig(t *testing.T) {
	const encodedSeed = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("RABBITMQ_URL", "amqp://x")
	t.Setenv("JWT_SIGNING_KEY", encodedSeed)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if output := fmt.Sprintf("%+v", cfg); strings.Contains(output, encodedSeed) {
		t.Fatalf("formatted config leaked signing key: %s", output)
	}
}
