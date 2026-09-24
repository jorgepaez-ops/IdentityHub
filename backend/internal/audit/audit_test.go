package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/api"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type recordingStore struct {
	params store.InsertAuditEventParams
}

func (s *recordingStore) InsertAuditEvent(_ context.Context, params store.InsertAuditEventParams) (store.AuditEvent, error) {
	s.params = params
	return store.AuditEvent{Action: params.Action}, nil
}

func TestRF011_RecordIncluyeContextoConfiable(t *testing.T) {
	actor := uuid.New()
	resourceType := "user"
	resourceID := uuid.NewString()
	metadata := json.RawMessage(`{"reason":"invalid_credentials"}`)
	clientIP := netip.MustParseAddr("203.0.113.17")
	store := &recordingStore{}

	handler := api.ClientIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		_, err := Record(request.Context(), store, request, Event{
			ActorUserID:  &actor,
			Action:       LoginFailed,
			ResourceType: &resourceType,
			ResourceID:   &resourceID,
			Metadata:     metadata,
		})
		if err != nil {
			t.Errorf("Record() error = %v", err)
		}
	}))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = clientIP.String() + ":443"
	request.Header.Set("User-Agent", "identity-hub-test")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if store.params.Action != string(LoginFailed) {
		t.Errorf("action = %q, want %q", store.params.Action, LoginFailed)
	}
	if store.params.ActorUserID == nil || *store.params.ActorUserID != actor {
		t.Errorf("actor = %v, want %v", store.params.ActorUserID, actor)
	}
	if store.params.IP == nil || *store.params.IP != clientIP {
		t.Errorf("IP = %v, want %v", store.params.IP, clientIP)
	}
	if store.params.UserAgent == nil || *store.params.UserAgent != "identity-hub-test" {
		t.Errorf("user agent = %v, want identity-hub-test", store.params.UserAgent)
	}
	if string(store.params.Metadata) != string(metadata) {
		t.Errorf("metadata = %s, want %s", store.params.Metadata, metadata)
	}
}
