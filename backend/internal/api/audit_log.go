package api

import (
	"net/http"
	"strconv"

	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
)

const defaultAuditLogLimit = 25

func (s *Server) listAuditLog(w http.ResponseWriter, r *http.Request, params ListAuditLogParams) {
	if s.auditLog == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit-log-unavailable", "Service Unavailable", "Audit log is temporarily unavailable.")
		return
	}
	limit := defaultAuditLogLimit
	if params.Limit != nil {
		limit = int(*params.Limit)
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	var cursor *int64
	if params.Cursor != nil {
		value, err := strconv.ParseInt(string(*params.Cursor), 10, 64)
		if err != nil || value < 1 {
			writeValidationProblem(w, "cursor", "must be a positive decimal audit event ID")
			return
		}
		cursor = &value
	}
	events, err := s.auditLog.List(r.Context(), auditlog.ListInput{
		Action: actionValue(params.Action), ActorID: params.ActorId, Since: params.Since, Cursor: cursor, Limit: limit + 1,
	})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "audit-log-load-failed", "Internal Server Error", "Audit log could not be loaded.")
		return
	}
	page := AuditLogPage{Items: make([]AuditEvent, 0, len(events))}
	if len(events) > limit {
		events = events[:limit]
		value := strconv.FormatInt(events[len(events)-1].ID, 10)
		page.NextCursor = &value
	}
	for _, event := range events {
		page.Items = append(page.Items, apiAuditEvent(event))
	}
	writeJSON(w, http.StatusOK, page)
}

func actionValue(value *Action) *string {
	if value == nil {
		return nil
	}
	result := string(*value)
	return &result
}

func apiAuditEvent(event auditlog.Event) AuditEvent {
	return AuditEvent{Id: event.ID, ActorUserId: event.ActorUserID, Action: event.Action, ResourceType: event.ResourceType, ResourceId: event.ResourceID, Ip: event.IP, UserAgent: event.UserAgent, Metadata: &event.Metadata, CreatedAt: event.CreatedAt}
}
