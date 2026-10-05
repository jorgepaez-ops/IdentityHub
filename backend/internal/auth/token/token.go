// Package token emite y valida access tokens Ed25519.
package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const accessTokenTTL = 15 * time.Minute

// AccessTokenExpiresIn is the access token lifetime in seconds, as reported in
// the `expiresIn` field of every token response.
const AccessTokenExpiresIn = int(accessTokenTTL / time.Second)

type Clock func() time.Time

type Claims struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions,omitempty"`
	SessionID   string   `json:"sid,omitempty"`
	jwt.RegisteredClaims
	permissionsPresent bool
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
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("ed25519 private key returned an unexpected public key type")
	}
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
	return s.issue(subject, roles, nil, "")
}

// IssueForSession binds a console access token to its refresh-token family so
// the session-management endpoint can identify the current session.
func (s *Service) IssueForSession(subject string, roles []string, sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("session ID is required")
	}
	return s.issue(subject, roles, nil, sessionID)
}

// IssueForAudience emits an application access token. It deliberately has no
// session claim because browser applications never receive Hub refresh state.
func (s *Service) IssueForAudience(subject string, roles, permissions []string, audience string) (string, error) {
	if audience == "" {
		return "", fmt.Errorf("jwt audience is required")
	}
	return s.issueForAudience(subject, roles, permissions, "", audience, true)
}

func (s *Service) issue(subject string, roles, permissions []string, sessionID string) (string, error) {
	return s.issueForAudience(subject, roles, permissions, sessionID, s.audience, false)
}

func (s *Service) issueForAudience(subject string, roles, permissions []string, sessionID, audience string, permissionsPresent bool) (string, error) {
	if subject == "" {
		return "", fmt.Errorf("jwt subject is required")
	}
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", fmt.Errorf("generate jwt ID: %w", err)
	}
	now := s.clock()
	claims := Claims{
		Roles:              append([]string(nil), roles...),
		Permissions:        append([]string{}, permissions...),
		SessionID:          sessionID,
		permissionsPresent: permissionsPresent,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{audience},
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

// MarshalJSON keeps the Hub claim set unchanged while requiring every
// application token to declare permissions, including an empty array.
func (c Claims) MarshalJSON() ([]byte, error) {
	type shared struct {
		Roles     []string `json:"roles"`
		SessionID string   `json:"sid,omitempty"`
		jwt.RegisteredClaims
	}
	base := shared{Roles: c.Roles, SessionID: c.SessionID, RegisteredClaims: c.RegisteredClaims}
	if !c.permissionsPresent {
		return json.Marshal(base)
	}
	return json.Marshal(struct {
		shared
		Permissions []string `json:"permissions"`
	}{shared: base, Permissions: c.Permissions})
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
