package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/invitation"
)

// acceptInvitation implements POST /api/v1/auth/invitations/accept (RF-002).
// An unknown, expired, or already-used token all return the same 410: which
// state was observed is never revealed, matching VerifyEmail's week-2
// behavior for the same reason (AM-004-style non-enumeration).
func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	if s.invitationAccept == nil {
		writeProblem(w, http.StatusServiceUnavailable, "invitation-acceptance-unavailable", "Service Unavailable", "Invitation acceptance is temporarily unavailable.")
		return
	}
	var request InvitationAcceptRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	err := s.invitationAccept.Accept(r.Context(), invitation.Input{
		Token:     request.Token,
		Password:  request.Password,
		IP:        requestClientIP(r),
		UserAgent: optionalRequestUserAgent(r),
	})
	var invalid *invitation.InvalidInputError
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, invitation.ErrInvalidInput):
		writeValidationProblem(w, "token", "must not be empty")
	case errors.As(err, &invalid):
		writeValidationProblem(w, invalid.Field, invalid.Detail)
	case errors.Is(err, invitation.ErrTokenInvalid):
		// Unknown, expired, and used tokens deliberately share this response
		// to avoid revealing which token state was observed.
		writeProblem(w, http.StatusGone, "invitation-token-unavailable", "Gone", "The invitation token is no longer available.")
	case errors.Is(err, invitation.ErrPublish):
		writeProblem(w, http.StatusServiceUnavailable, "event-unavailable", "Service Unavailable", "Invitation acceptance is temporarily unavailable.")
	default:
		writeProblem(w, http.StatusInternalServerError, "invitation-acceptance-failed", "Internal Server Error", "The invitation could not be accepted.")
	}
}
