package token

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testSeed() []byte { return []byte("01234567890123456789012345678901") }

func TestRF004_VigenciaDe15Minutos(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	service, err := New(testSeed(), "issuer", "audience", func() time.Time { return now })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	raw, err := service.Issue("user-1", []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := service.Validate(raw)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time); got != 15*time.Minute {
		t.Fatalf("exp - iat = %s, want 15m", got)
	}

	expired := now.Add(16 * time.Minute)
	expiredService, err := New(testSeed(), "issuer", "audience", func() time.Time { return expired })
	if err != nil {
		t.Fatalf("New expired service: %v", err)
	}
	if _, err := expiredService.Validate(raw); err == nil {
		t.Fatal("Validate accepted an expired token")
	}
}

func TestRF004_TokenSeVerificaConLaClavePublica(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	service, err := New(testSeed(), "issuer", "audience", func() time.Time { return now })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := service.Issue("user-1", []string{"user"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	parsed, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		return service.PublicKey(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}), jwt.WithIssuer("issuer"), jwt.WithAudience("audience"), jwt.WithExpirationRequired(), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || !parsed.Valid {
		t.Fatalf("public key verification failed: %v", err)
	}
}

func TestRF004_RechazaEmisorYAudienciaIncorrectos(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	issuer, err := New(testSeed(), "issuer", "audience", func() time.Time { return now })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := issuer.Issue("user-1", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	wrongIssuer, err := New(testSeed(), "other-issuer", "audience", func() time.Time { return now })
	if err != nil {
		t.Fatalf("New wrong issuer: %v", err)
	}
	if _, err := wrongIssuer.Validate(raw); err == nil {
		t.Fatal("Validate accepted wrong issuer")
	}
	wrongAudience, err := New(testSeed(), "issuer", "other-audience", func() time.Time { return now })
	if err != nil {
		t.Fatalf("New wrong audience: %v", err)
	}
	if _, err := wrongAudience.Validate(raw); err == nil {
		t.Fatal("Validate accepted wrong audience")
	}
}

func TestRF004_RechazaTokenSinSujetoOFirmadoPorOtraClave(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	service, err := New([]byte("01234567890123456789012345678901"), "issuer", "audience", now)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	build := func(subject string, key ed25519.PrivateKey) string {
		claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "issuer", Subject: subject, Audience: jwt.ClaimStrings{"audience"},
			ExpiresAt: jwt.NewNumericDate(now().Add(15 * time.Minute)), IssuedAt: jwt.NewNumericDate(now()),
		}}
		signed := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		signed.Header["kid"] = service.KeyID()
		raw, err := signed.SignedString(key)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return raw
	}
	ownKey := ed25519.NewKeyFromSeed([]byte("01234567890123456789012345678901"))
	otherKey := ed25519.NewKeyFromSeed([]byte("abcdefghijklmnopqrstuvwxyz012345"))

	if _, err := service.Validate(build("user-1", ownKey)); err != nil {
		t.Fatalf("control token rejected: %v", err)
	}
	if _, err := service.Validate(build("", ownKey)); err == nil {
		t.Error("token without subject was accepted")
	}
	if _, err := service.Validate(build("user-1", otherKey)); err == nil {
		t.Error("token signed by another key with the right kid was accepted")
	}
}
