// Package config carga la configuración desde el entorno.
//
// Principio de diseño (RNF-003): un secreto ausente es un error de arranque, no
// un valor por defecto. Una aplicación que arranca con una contraseña por
// defecto es peor que una que no arranca, porque nadie se entera.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Secret envuelve un valor sensible para que no aparezca en los logs.
// Cumple RNF-012 / AM-013: formatear la estructura que lo contiene imprime
// "[REDACTADO]" en lugar del valor.
type Secret string

func (s Secret) String() string               { return "[REDACTADO]" }
func (s Secret) GoString() string             { return "[REDACTADO]" }
func (s Secret) Reveal() string               { return string(s) }
func (s Secret) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTADO]"`), nil }

type PasswordConfig struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	Concurrency int
}

type Config struct {
	Port        int
	LogLevel    string
	Version     string
	DatabaseURL Secret
	RabbitURL   Secret
	SMTPHost    string
	SMTPPort    int
	SMTPFrom    string
	JWTIssuer   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	Password    PasswordConfig
}

// Load lee el entorno y acumula TODOS los errores antes de fallar, en vez de
// abortar en el primero. Arrancar el contenedor cinco veces para descubrir cinco
// variables faltantes es una forma tonta de perder una tarde.
func Load() (*Config, error) {
	var problems []string

	req := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			problems = append(problems, fmt.Sprintf("falta la variable obligatoria %s", key))
		}
		return v
	}
	opt := func(key, def string) string {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
		return def
	}
	dur := func(key, def string) time.Duration {
		d, err := time.ParseDuration(opt(key, def))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s no es una duración válida: %v", key, err))
		}
		return d
	}
	num := func(key, def string) int {
		n, err := strconv.Atoi(opt(key, def))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s no es un entero válido: %v", key, err))
		}
		return n
	}
	passwordNum := func(key, def string, max int) int {
		n := num(key, def)
		if n <= 0 || n > max {
			problems = append(problems, fmt.Sprintf("%s debe estar entre 1 y %d", key, max))
		}
		return n
	}

	passwordMemory := passwordNum("ARGON2_MEMORY_KIB", "65536", int(^uint32(0)))
	passwordIterations := passwordNum("ARGON2_ITERATIONS", "3", int(^uint32(0)))
	passwordParallelism := passwordNum("ARGON2_PARALLELISM", "2", 255)
	passwordConcurrency := passwordNum("ARGON2_CONCURRENCY", "4", int(^uint(0)>>1))

	cfg := &Config{
		Port:        num("API_PORT", "8081"),
		LogLevel:    opt("LOG_LEVEL", "info"),
		Version:     opt("APP_VERSION", "dev"),
		DatabaseURL: Secret(req("DATABASE_URL")),
		RabbitURL:   Secret(req("RABBITMQ_URL")),
		SMTPHost:    opt("SMTP_HOST", "mailpit"),
		SMTPPort:    num("SMTP_PORT", "1025"),
		SMTPFrom:    opt("SMTP_FROM", "no-reply@identity.local"),
		JWTIssuer:   opt("JWT_ISSUER", "http://localhost:8080"),
		AccessTTL:   dur("JWT_ACCESS_TTL", "15m"),
		RefreshTTL:  dur("JWT_REFRESH_TTL", "720h"),
		Password: PasswordConfig{
			MemoryKiB:   uint32(passwordMemory),
			Iterations:  uint32(passwordIterations),
			Parallelism: uint8(passwordParallelism),
			Concurrency: passwordConcurrency,
		},
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("configuración inválida:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}
