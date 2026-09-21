package api

import (
	"encoding/base64"
	"net/http"
)

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

func (s *Server) GetJwks(w http.ResponseWriter, _ *http.Request) {
	if s.tokens == nil {
		writeProblem(w, http.StatusServiceUnavailable, "service-unavailable", "Service Unavailable", "The service is not ready.")
		return
	}
	writeJSON(w, http.StatusOK, jwksDocument{Keys: []jwk{{
		Kty: "OKP", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(s.tokens.PublicKey()),
		Kid: s.tokens.KeyID(), Use: "sig", Alg: "EdDSA",
	}}})
}
