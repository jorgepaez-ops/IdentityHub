package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
)

const defaultAuditLogLimit = 25

func (s *Server) listAuditLog(w http.ResponseWriter, r *http.Request, params ListAuditLogParams) {
	if s.auditLog == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit-log-unavailable", titleServiceUnavailable, "Audit log is temporarily unavailable.")
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
		writeProblem(w, http.StatusInternalServerError, "audit-log-load-failed", titleInternalServerError, "Audit log could not be loaded.")
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

// auditExportColumns is the fixed CSV header of RF-019.
var auditExportColumns = []string{"id", "created_at", "actor_user_id", "action", "resource_type", "resource_id", "ip", "user_agent", "metadata"}

func (s *Server) exportAuditLog(w http.ResponseWriter, r *http.Request, params ExportAuditLogParams) {
	if s.auditLog == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit-log-unavailable", titleServiceUnavailable, "Audit log is temporarily unavailable.")
		return
	}
	if !params.Format.Valid() {
		writeValidationProblem(w, "format", "must be csv or json")
		return
	}
	actorID, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	result, err := s.auditLog.Export(r.Context(), auditlog.ExportInput{
		Action: actionValue(params.Action), ActorID: params.ActorId, Since: params.Since, Format: string(params.Format),
		RequestedBy: actorID, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r),
	})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "audit-log-export-failed", titleInternalServerError, "Audit log could not be exported.")
		return
	}
	extension, contentType := "json", "application/json; charset=utf-8"
	if params.Format == ExportAuditLogParamsFormatCsv {
		extension, contentType = "csv", "text/csv; charset=utf-8"
	}
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="audit-log-%s.%s"`, time.Now().UTC().Format("20060102T150405Z"), extension))
	h.Set("X-Export-Truncated", strconv.FormatBool(result.Truncated))
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if params.Format == ExportAuditLogParamsFormatCsv {
		writeAuditCSV(w, result.Events)
		return
	}
	items := make([]AuditEvent, 0, len(result.Events))
	for _, event := range result.Events {
		items = append(items, apiAuditEvent(event))
	}
	_ = json.NewEncoder(w).Encode(items)
}

// writeAuditCSV writes a UTF-8 BOM (Excel reads the accents) and CRLF-terminated records. The status
// is already sent, so a write error can only mean the client went away.
func writeAuditCSV(w http.ResponseWriter, events []auditlog.Event) {
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	out := csv.NewWriter(w)
	out.UseCRLF = true
	_ = out.Write(auditExportColumns)
	for _, event := range events {
		metadata, err := json.Marshal(event.Metadata)
		if err != nil {
			metadata = []byte("{}")
		}
		actor := ""
		if event.ActorUserID != nil {
			actor = event.ActorUserID.String()
		}
		record := []string{
			strconv.FormatInt(event.ID, 10), event.CreatedAt.UTC().Format(time.RFC3339), actor, event.Action,
			deref(event.ResourceType), deref(event.ResourceID), deref(event.IP), deref(event.UserAgent), string(metadata),
		}
		for i, cell := range record {
			record[i] = neutralizeCSVCell(cell)
		}
		_ = out.Write(record)
	}
	out.Flush()
}

// neutralizeCSVCell stops spreadsheets from running a cell as a formula (OWASP CSV injection).
func neutralizeCSVCell(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
