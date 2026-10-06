package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/session"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type sessionManager interface {
	List(context.Context, uuid.UUID) ([]session.Session, error)
	Revoke(context.Context, uuid.UUID, uuid.UUID) error
}

// ListSessions returns the authenticated subject's active refresh-token
// families. The current marker comes from the signed session-family claim.
func (s *Server) ListSessions(w http.ResponseWriter, r *http.Request) {
	s.withSessionSubject(s.listSessions).ServeHTTP(w, r)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	if s.sessions == nil {
		writeProblem(w, http.StatusServiceUnavailable, "sessions-unavailable", titleServiceUnavailable, "Sessions are temporarily unavailable.")
		return
	}
	items, err := s.sessions.List(r.Context(), userID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "sessions-load-failed", titleInternalServerError, "Sessions could not be loaded.")
		return
	}
	claims, _ := ClaimsFrom(r.Context())
	currentFamilyID, _ := uuid.Parse(claims.SessionID)
	response := make([]Session, 0, len(items))
	for _, item := range items {
		ip := ""
		if item.IP != nil {
			ip = item.IP.String()
		}
		response = append(response, Session{Id: openapi_types.UUID(item.ID), Ip: ip, UserAgent: item.UserAgent, CreatedAt: item.CreatedAt, LastUsedAt: item.LastUsedAt, Current: item.ID == currentFamilyID})
	}
	writeJSON(w, http.StatusOK, response)
}

// RevokeSession revokes one active family only when it belongs to the signed-in
// subject. Missing and foreign families deliberately share the same 404.
func (s *Server) RevokeSession(w http.ResponseWriter, r *http.Request, sessionID SessionId) {
	s.withSessionSubject(func(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
		if s.sessions == nil {
			writeProblem(w, http.StatusServiceUnavailable, "sessions-unavailable", titleServiceUnavailable, "Sessions are temporarily unavailable.")
			return
		}
		if err := s.sessions.Revoke(r.Context(), userID, uuid.UUID(sessionID)); err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
				writeProblem(w, http.StatusNotFound, "session-not-found", "Not Found", "The session was not found.")
				return
			}
			writeProblem(w, http.StatusInternalServerError, "session-revoke-failed", titleInternalServerError, "The session could not be revoked.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

func (s *Server) withSessionSubject(next func(http.ResponseWriter, *http.Request, uuid.UUID)) http.Handler {
	return RequireAuth(s.tokens)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := currentUserID(r)
		if !ok {
			writeUnauthorized(w)
			return
		}
		next(w, r, userID)
	}))
}
