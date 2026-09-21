package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

func apiTestService(t *testing.T) *token.Service {
	t.Helper()
	service, err := token.New([]byte("01234567890123456789012345678901"), "issuer", "audience", func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) })
	if err != nil {
		t.Fatalf("token.New: %v", err)
	}
	return service
}

func TestRF004_TokenSeVerificaConLaClaveDelJWKS(t *testing.T) {
	service := apiTestService(t)
	server := NewServer(nil, "test", nil)
	server.SetTokenService(service)
	response := httptest.NewRecorder()
	server.GetJwks(response, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GetJwks status = %d", response.Code)
	}
	var document struct {
		Keys []struct{ Kty, Crv, X, Kid, Use, Alg string } `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		t.Fatalf("decode JWKS: %v", err)
	}
	if len(document.Keys) != 1 {
		t.Fatalf("JWKS keys = %d, want 1", len(document.Keys))
	}
	key := document.Keys[0]
	public, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		t.Fatalf("decode x: %v", err)
	}
	raw, err := service.Issue("user-1", []string{"user"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	parsed, err := jwt.Parse(raw, func(*jwt.Token) (any, error) { return ed25519.PublicKey(public), nil }, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer("issuer"), jwt.WithAudience("audience"), jwt.WithExpirationRequired(), jwt.WithTimeFunc(func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }))
	if err != nil || !parsed.Valid {
		t.Fatalf("JWKS key did not verify token: %v", err)
	}
	if key.Kty != "OKP" || key.Crv != "Ed25519" || key.Use != "sig" || key.Alg != "EdDSA" || key.Kid != service.KeyID() {
		t.Fatalf("invalid JWKS key: %+v", key)
	}
}

func TestRF004_RechazaTokenConAlgoritmoAlterado(t *testing.T) {
	service := apiTestService(t)
	protected := RequireAuth(service)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	// Same kid, issuer, audience, subject and expiry as a valid token: only the
	// algorithm differs, so a 401 can only come from the algorithm restriction.
	claims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss": "issuer", "aud": "audience", "sub": "user-1", "roles": []string{"admin"},
			"iat": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Unix(),
			"exp": time.Date(2026, 1, 2, 3, 19, 5, 0, time.UTC).Unix(),
		}
	}
	none := jwt.NewWithClaims(jwt.SigningMethodNone, claims())
	none.Header["kid"] = service.KeyID()
	noneRaw, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("none token: %v", err)
	}
	hmac := jwt.NewWithClaims(jwt.SigningMethodHS256, claims())
	hmac.Header["kid"] = service.KeyID()
	hmacRaw, err := hmac.SignedString([]byte(service.PublicKey()))
	if err != nil {
		t.Fatalf("HS256 token: %v", err)
	}
	for name, raw := range map[string]string{"alg none": noneRaw, "HS256 signed with the public key": hmacRaw} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/protected", nil)
		request.Header.Set("Authorization", "Bearer "+raw)
		protected.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", name, response.Code)
		}
		if got := response.Header().Get("Content-Type"); got != "application/problem+json; charset=utf-8" {
			t.Fatalf("%s: Content-Type = %q", name, got)
		}
	}

	// Control: the same claims and kid signed with EdDSA by the service pass.
	valid, err := service.Issue("user-1", []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+valid)
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid EdDSA token: status = %d, want 204", response.Code)
	}
}

func TestRF004_ServicioSinClaveFallaDeFormaSegura(t *testing.T) {
	server := NewServer(nil, "test", nil)
	response := httptest.NewRecorder()
	server.GetJwks(response, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("GetJwks without token service status = %d, want 503", response.Code)
	}

	response = httptest.NewRecorder()
	RequireAuth(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("RequireAuth without token service status = %d, want 401", response.Code)
	}
}
