// Package config carga la configuración desde el entorno.
//
// Principio de diseño (RNF-003): un secreto ausente es un error de arranque, no
// un valor por defecto. Una aplicación que arranca con una contraseña por
// defecto es peor que una que no arranca, porque nadie se entera.
package config

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/netip"
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
	Port           int
	LogLevel       string
	Version        string
	DatabaseURL    Secret
	RabbitURL      Secret
	SMTPHost       string
	SMTPPort       int
	SMTPFrom       string
	JWTSigningKey  Secret
	JWTIssuer      string
	JWTAudience    string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	Argon2         PasswordConfig
	TrustedProxies []netip.Prefix
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

	jwtSigningKey := req("JWT_SIGNING_KEY")
	if decoded, err := base64.StdEncoding.DecodeString(jwtSigningKey); err != nil || len(decoded) != 32 {
		problems = append(problems, "JWT_SIGNING_KEY debe ser una semilla Ed25519 en base64 de 32 bytes")
	}

	passwordMemory := passwordNum("ARGON2_MEMORY_KIB", "65536", int(^uint32(0)))
	passwordIterations := passwordNum("ARGON2_ITERATIONS", "3", int(^uint32(0)))
	passwordParallelism := passwordNum("ARGON2_PARALLELISM", "2", 255)
	passwordConcurrency := passwordNum("ARGON2_CONCURRENCY", "4", int(^uint(0)>>1))
	trustedProxies := parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"), &problems)

	cfg := &Config{
		Port:           num("API_PORT", "8081"),
		LogLevel:       opt("LOG_LEVEL", "info"),
		Version:        opt("APP_VERSION", "dev"),
		DatabaseURL:    Secret(req("DATABASE_URL")),
		RabbitURL:      Secret(req("RABBITMQ_URL")),
		SMTPHost:       opt("SMTP_HOST", "mailpit"),
		SMTPPort:       num("SMTP_PORT", "1025"),
		SMTPFrom:       opt("SMTP_FROM", "no-reply@identity.local"),
		JWTSigningKey:  Secret(jwtSigningKey),
		JWTIssuer:      opt("JWT_ISSUER", "http://localhost:8080"),
		JWTAudience:    opt("JWT_AUDIENCE", "identity-hub"),
		AccessTTL:      dur("JWT_ACCESS_TTL", "15m"),
		RefreshTTL:     dur("JWT_REFRESH_TTL", "720h"),
		TrustedProxies: trustedProxies,
		Argon2: PasswordConfig{
			MemoryKiB:   boundedUint32(passwordMemory),
			Iterations:  boundedUint32(passwordIterations),
			Parallelism: boundedUint8(passwordParallelism),
			Concurrency: passwordConcurrency,
		},
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("configuración inválida:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// boundedUint32 and boundedUint8 narrow values that passwordNum has already
// validated; the explicit range check makes the conversion safe on its own.
func boundedUint32(n int) uint32 {
	if n < 0 || n > math.MaxUint32 {
		return 0
	}
	return uint32(n)
}

func boundedUint8(n int) uint8 {
	if n < 0 || n > math.MaxUint8 {
		return 0
	}
	return uint8(n)
}

func parseTrustedProxies(value string, problems *[]string) []netip.Prefix {
	var prefixes []netip.Prefix
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			*problems = append(*problems, fmt.Sprintf("TRUSTED_PROXIES contiene un CIDR inválido %q: %v", entry, err))
			continue
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}
