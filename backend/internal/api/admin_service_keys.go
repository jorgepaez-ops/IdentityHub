package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/servicekey"
)

// Handlers of service key administration (RF-018, ADR 0014). They are reached only through the
// Server methods wrapped with adminOnly: a service key can never issue, list or revoke keys.

func (s *Server) serviceKeysReady(w http.ResponseWriter) bool {
	if s.serviceKeys == nil {
		writeProblem(w, http.StatusServiceUnavailable, "service-keys-unavailable", titleServiceUnavailable, "Service keys are temporarily unavailable.")
		return false
	}
	return true
}

func (s *Server) createServiceKey(w http.ResponseWriter, r *http.Request) {
	if !s.serviceKeysReady(w) {
		return
	}
	actor, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request CreateServiceKeyRequest
	if !decodeRoleGridRequest(w, r, &request) {
		return
	}
	input := servicekey.CreateInput{ActorUserID: actor, Name: request.Name, ExpiresInDays: request.ExpiresInDays, UserAgent: r.UserAgent()}
	for _, scope := range request.Scopes {
		input.Scopes = append(input.Scopes, string(scope))
	}
	if ip := requestClientIP(r); ip != nil {
		input.IP = *ip
	}
	created, err := s.serviceKeys.Create(r.Context(), input)
	if err != nil {
		var invalid *servicekey.InputError
		if errors.As(err, &invalid) {
			writeValidationProblem(w, invalid.Field, invalid.Detail)
			return
		}
		s.logRoleGridError(r, "service key creation failed", err)
		writeProblem(w, http.StatusInternalServerError, "service-key-create-failed", titleInternalServerError, "The service key could not be created.")
		return
	}
	// The complete key is in this response only; it cannot be retrieved again.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, CreatedServiceKey{
		Key: created.FullKey, Id: created.ID, Name: created.Name, Scopes: apiServiceKeyScopes(created.Scopes), Prefix: created.Prefix(),
		CreatedAt: created.CreatedAt, CreatedBy: created.CreatedBy, ExpiresAt: created.ExpiresAt, LastUsedAt: created.LastUsedAt, RevokedAt: created.RevokedAt,
	})
}

func (s *Server) listServiceKeys(w http.ResponseWriter, r *http.Request) {
	if !s.serviceKeysReady(w) {
		return
	}
	keys, err := s.serviceKeys.List(r.Context())
	if err != nil {
		s.logRoleGridError(r, "service key list failed", err)
		writeProblem(w, http.StatusInternalServerError, "service-keys-load-failed", titleInternalServerError, "Service keys could not be loaded.")
		return
	}
	result := make([]ServiceKey, 0, len(keys))
	for _, key := range keys {
		result = append(result, ServiceKey{
			Id: key.ID, Name: key.Name, Scopes: apiServiceKeyScopes(key.Scopes), Prefix: key.Prefix(),
			CreatedAt: key.CreatedAt, CreatedBy: key.CreatedBy, ExpiresAt: key.ExpiresAt, LastUsedAt: key.LastUsedAt, RevokedAt: key.RevokedAt,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) revokeServiceKey(w http.ResponseWriter, r *http.Request, serviceKeyID ServiceKeyId) {
	if !s.serviceKeysReady(w) {
		return
	}
	actor, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	input := servicekey.RevokeInput{ActorUserID: actor, UserAgent: r.UserAgent()}
	if ip := requestClientIP(r); ip != nil {
		input.IP = *ip
	}
	if err := s.serviceKeys.Revoke(r.Context(), uuid.UUID(serviceKeyID), input); err != nil {
		if errors.Is(err, servicekey.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "service-key-not-found", "Not Found", "The service key was not found.")
			return
		}
		s.logRoleGridError(r, "service key revocation failed", err)
		writeProblem(w, http.StatusInternalServerError, "service-key-revoke-failed", titleInternalServerError, "The service key could not be revoked.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func apiServiceKeyScopes(scopes []string) []ServiceKeyScope {
	result := make([]ServiceKeyScope, 0, len(scopes))
	for _, scope := range scopes {
		result = append(result, ServiceKeyScope(scope))
	}
	return result
}
