package api

import (
	"encoding/json"
	"net/http"
	"net/netip"
)

func optionalRequestUserAgent(r *http.Request) *string {
	if value := r.UserAgent(); value != "" {
		return &value
	}
	return nil
}

func writeValidationProblem(w http.ResponseWriter, field, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "https://identity.local/problems/validation", "title": "Bad Request", "status": http.StatusBadRequest, "detail": "Request validation failed.", "errors": []map[string]string{{"field": field, "detail": detail}}})
}

func requestClientIP(r *http.Request) *netip.Addr {
	address, ok := ClientIPFrom(r.Context())
	if !ok {
		return nil
	}
	return &address
}
