package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/invitationresend"
)

func (s *Server) resendInvitation(w http.ResponseWriter, r *http.Request, userID UserId) {
	if s.invitationResend == nil {
		writeProblem(w, http.StatusServiceUnavailable, "invitation-resend-unavailable", titleServiceUnavailable, "Invitation resend is temporarily unavailable.")
		return
	}
	actorID, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	err := s.invitationResend.Resend(r.Context(), invitationresend.Input{ActorUserID: actorID, UserID: uuid.UUID(userID), IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, invitationresend.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "user-not-found", "Not Found", "The user was not found.")
	case errors.Is(err, invitationresend.ErrAccountNotPending):
		writeProblem(w, http.StatusConflict, "invitation-not-available", "Conflict", "The user is not pending verification.")
	case errors.Is(err, invitationresend.ErrPublish):
		writeProblem(w, http.StatusServiceUnavailable, "event-unavailable", titleServiceUnavailable, "Invitation resend is temporarily unavailable.")
	default:
		writeProblem(w, http.StatusInternalServerError, "invitation-resend-failed", titleInternalServerError, "The invitation could not be resent.")
	}
}
