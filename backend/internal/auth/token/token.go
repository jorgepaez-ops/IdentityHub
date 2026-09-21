// Package token emite y valida access tokens Ed25519.
package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const accessTokenTTL = 15 * time.Minute

type Clock func() time.Time

type Claims struct {
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

type Service struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	issuer     string
	audience   string
	keyID      string
	clock      Clock
}

func New(seed []byte, issuer, audience string, clock Clock) (*Service, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("ed25519 seed must contain %d bytes", ed25519.SeedSize)
	}
	if issuer == "" {
		return nil, fmt.Errorf("jwt issuer is required")
	}
	if audience == "" {
		return nil, fmt.Errorf("jwt audience is required")
	}
	if clock == nil {
		clock = time.Now
	}

	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	fingerprint := sha256.Sum256(publicKey)
	return &Service{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     issuer,
		audience:   audience,
		keyID:      hex.EncodeToString(fingerprint[:])[:16],
		clock:      clock,
	}, nil
}

func (s *Service) Issue(subject string, roles []string) (string, error) {
	if subject == "" {
		return "", fmt.Errorf("jwt subject is required")
	}
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", fmt.Errorf("generate jwt ID: %w", err)
	}
	now := s.clock()
	claims := Claims{
		Roles: append([]string(nil), roles...),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        hex.EncodeToString(jti),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.keyID
	raw, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return raw, nil
}

func (s *Service) Validate(raw string) (Claims, error) {
	claims := Claims{}
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(candidate *jwt.Token) (any, error) {
		if kid, _ := candidate.Header["kid"].(string); kid != s.keyID {
			return nil, fmt.Errorf("unknown jwt key ID")
		}
		return s.publicKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}), jwt.WithIssuer(s.issuer), jwt.WithAudience(s.audience), jwt.WithExpirationRequired(), jwt.WithTimeFunc(s.clock))
	if err != nil {
		return Claims{}, fmt.Errorf("validate access token: %w", err)
	}
	if !parsed.Valid {
		return Claims{}, fmt.Errorf("validate access token: token is invalid")
	}
	if claims.Subject == "" {
		return Claims{}, fmt.Errorf("validate access token: subject is required")
	}
	return claims, nil
}

func (s *Service) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), s.publicKey...)
}
func (s *Service) KeyID() string { return s.keyID }
