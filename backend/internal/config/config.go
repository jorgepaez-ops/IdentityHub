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
	"net/mail"
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
	Port                    int
	LogLevel                string
	Version                 string
	DatabaseURL             Secret
	RabbitURL               Secret
	SMTPHost                string
	SMTPPort                int
	SMTPFrom                string
	PublicBaseURL           string
	JWTSigningKey           Secret
	JWTIssuer               string
	JWTAudience             string
	AccessTTL               time.Duration
	RefreshTTL              time.Duration
	HubSessionTTL           time.Duration
	OAuthClientID           string
	OAuthRedirectURI        string
	OAuthClientOrigin       string
	LoginAccountMaxFailures int
	LoginIPMaxFailures      int
	LoginFailureWindow      time.Duration
	LoginLockoutDuration    time.Duration
	Argon2                  PasswordConfig
	TrustedProxies          []netip.Prefix
	BootstrapAdminEmail     string
}

const (
	OAuthClientID     = "contabilidad"
	OAuthRedirectURI  = "http://contabilidad.localhost:8080/oauth/callback"
	OAuthClientOrigin = "http://contabilidad.localhost:8080"
)

// Load lee el entorno y acumula TODOS los errores antes de fallar, en vez de
// abortar en el primero. Arrancar el contenedor cinco veces para descubrir cinco
// variables faltantes es una forma tonta de perder una tarde.
func Load() (*Config, error) {
	return load(true)
}

// LoadWorker loads the configuration of the worker binary. It is identical to
// Load except that neither DATABASE_URL nor JWT_SIGNING_KEY is required or read:
// the worker only consumes the queue and sends SMTP, so it never opens a
// database connection nor signs or verifies tokens, and should not be handed
// those credentials (least privilege).
func LoadWorker() (*Config, error) {
	return load(false)
}

// problemCollector accumulates every configuration problem, in the order the
// variables are read, so load can report them all at once.
type problemCollector struct {
	problems []string
}

func (c *problemCollector) add(format string, args ...any) {
	c.problems = append(c.problems, fmt.Sprintf(format, args...))
}

// required reads a trimmed variable and records a problem when it is empty.
func (c *problemCollector) required(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		c.add("falta la variable obligatoria %s", key)
	}
	return v
}

// optional reads a trimmed variable and falls back to def when it is empty.
func (c *problemCollector) optional(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func (c *problemCollector) duration(key, def string) time.Duration {
	d, err := time.ParseDuration(c.optional(key, def))
	if err != nil {
		c.add("%s no es una duración válida: %v", key, err)
	}
	return d
}

func (c *problemCollector) integer(key, def string) int {
	n, err := strconv.Atoi(c.optional(key, def))
	if err != nil {
		c.add("%s no es un entero válido: %v", key, err)
	}
	return n
}

// boundedInteger records a problem unless the value is in 1..upperBound.
func (c *problemCollector) boundedInteger(key, def string, upperBound int) int {
	n := c.integer(key, def)
	if n <= 0 || n > upperBound {
		c.add("%s debe estar entre 1 y %d", key, upperBound)
	}
	return n
}

func (c *problemCollector) positiveInteger(key, def string) int {
	n := c.integer(key, def)
	if n <= 0 {
		c.add("%s debe ser mayor que 0", key)
	}
	return n
}

func (c *problemCollector) positiveDuration(key, def string) time.Duration {
	d := c.duration(key, def)
	if d <= 0 {
		c.add("%s debe ser una duración mayor que 0", key)
	}
	return d
}

func load(requireDatabase bool) (*Config, error) {
	c := &problemCollector{}

	databaseURL, jwtSigningKey := loadServerSecrets(c, requireDatabase)
	argon2 := loadPasswordConfig(c)
	trustedProxies := parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"), &c.problems)
	bootstrapAdminEmail := loadBootstrapAdminEmail(c)

	cfg := &Config{
		Port:                    c.integer("API_PORT", "8081"),
		LogLevel:                c.optional("LOG_LEVEL", "info"),
		Version:                 c.optional("APP_VERSION", "dev"),
		DatabaseURL:             Secret(databaseURL),
		RabbitURL:               Secret(c.required("RABBITMQ_URL")),
		SMTPHost:                c.optional("SMTP_HOST", "mailpit"),
		SMTPPort:                c.integer("SMTP_PORT", "1025"),
		SMTPFrom:                c.optional("SMTP_FROM", "no-reply@identity.local"),
		PublicBaseURL:           c.optional("PUBLIC_BASE_URL", "http://identityhub.localhost:8080"),
		JWTSigningKey:           Secret(jwtSigningKey),
		JWTIssuer:               c.optional("JWT_ISSUER", "http://identityhub.localhost:8080"),
		JWTAudience:             c.optional("JWT_AUDIENCE", "identity-hub"),
		AccessTTL:               c.duration("JWT_ACCESS_TTL", "15m"),
		RefreshTTL:              c.duration("JWT_REFRESH_TTL", "720h"),
		HubSessionTTL:           c.positiveDuration("HUB_SESSION_TTL", "720h"),
		OAuthClientID:           OAuthClientID,
		OAuthRedirectURI:        OAuthRedirectURI,
		OAuthClientOrigin:       OAuthClientOrigin,
		LoginAccountMaxFailures: c.positiveInteger("LOGIN_ACCOUNT_MAX_FAILURES", "5"),
		LoginIPMaxFailures:      c.positiveInteger("LOGIN_IP_MAX_FAILURES", "20"),
		LoginFailureWindow:      c.positiveDuration("LOGIN_FAILURE_WINDOW", "15m"),
		LoginLockoutDuration:    c.positiveDuration("LOGIN_LOCKOUT_DURATION", "15m"),
		TrustedProxies:          trustedProxies,
		BootstrapAdminEmail:     bootstrapAdminEmail,
		Argon2:                  argon2,
	}

	if len(c.problems) > 0 {
		return nil, fmt.Errorf("configuración inválida:\n  - %s", strings.Join(c.problems, "\n  - "))
	}
	return cfg, nil
}

// loadServerSecrets reads DATABASE_URL and JWT_SIGNING_KEY. The worker
// (requireDatabase false) never reads either, even if the environment sets them.
func loadServerSecrets(c *problemCollector, requireDatabase bool) (databaseURL, jwtSigningKey string) {
	if !requireDatabase {
		return "", ""
	}
	databaseURL = c.required("DATABASE_URL")
	jwtSigningKey = c.required("JWT_SIGNING_KEY")
	if decoded, err := base64.StdEncoding.DecodeString(jwtSigningKey); err != nil || len(decoded) != 32 {
		c.add("JWT_SIGNING_KEY debe ser una semilla Ed25519 en base64 de 32 bytes")
	}
	return databaseURL, jwtSigningKey
}

// loadPasswordConfig reads and validates the Argon2 parameters.
func loadPasswordConfig(c *problemCollector) PasswordConfig {
	memory := c.boundedInteger("ARGON2_MEMORY_KIB", "65536", int(^uint32(0)))
	iterations := c.boundedInteger("ARGON2_ITERATIONS", "3", int(^uint32(0)))
	parallelism := c.boundedInteger("ARGON2_PARALLELISM", "2", 255)
	concurrency := c.boundedInteger("ARGON2_CONCURRENCY", "4", int(^uint(0)>>1))
	return PasswordConfig{
		MemoryKiB:   boundedUint32(memory),
		Iterations:  boundedUint32(iterations),
		Parallelism: boundedUint8(parallelism),
		Concurrency: concurrency,
	}
}

// loadBootstrapAdminEmail reads the optional bootstrap admin address and
// records a problem when it is defined but not a plain valid email.
func loadBootstrapAdminEmail(c *problemCollector) string {
	email := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))
	if email == "" {
		return email
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		c.add("BOOTSTRAP_ADMIN_EMAIL debe ser un correo válido cuando está definido")
	}
	return email
}

// boundedUint32 and boundedUint8 narrow values that boundedInteger has already
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
