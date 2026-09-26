package auditlog

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type auditLogRepositoryStub struct {
	input  ListInput
	events []Event
	err    error
	calls  int
}

func (r *auditLogRepositoryStub) ListAuditLog(_ context.Context, input ListInput) ([]Event, error) {
	r.calls++
	r.input = input
	return r.events, r.err
}

func TestRNF005_AuditLogListBoundsPageSize(t *testing.T) {
	for _, tt := range []struct {
		name  string
		limit int
		want  int
	}{
		{name: "minimum page size", limit: 0, want: 1},
		{name: "public maximum plus successor", limit: 102, want: 101},
		{name: "valid page size is retained", limit: 35, want: 35},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := &auditLogRepositoryStub{events: []Event{{Action: "login_succeeded"}}}
			events, err := New(repository).List(context.Background(), ListInput{Limit: tt.limit})
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if repository.calls != 1 || repository.input.Limit != tt.want {
				t.Fatalf("repository calls=%d limit=%d, want 1 and %d", repository.calls, repository.input.Limit, tt.want)
			}
			if !reflect.DeepEqual(events, repository.events) {
				t.Fatalf("List() events = %#v, want %#v", events, repository.events)
			}
		})
	}
}

func TestRNF005_AuditLogListReportsUnavailableAndRepositoryErrors(t *testing.T) {
	if _, err := New(nil).List(context.Background(), ListInput{}); err == nil {
		t.Fatal("List() error = nil, want unavailable service error")
	}

	repositoryErr := errors.New("repository failure")
	_, err := New(&auditLogRepositoryStub{err: repositoryErr}).List(context.Background(), ListInput{Limit: 1})
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("List() error = %v, want wrapped %v", err, repositoryErr)
	}
}

func TestRNF005_DecodeAuditMetadataAcceptsObjectsAndRejectsInvalidJSON(t *testing.T) {
	metadata, err := DecodeMetadata([]byte(`{"source":"login","attempts":2}`))
	if err != nil {
		t.Fatalf("DecodeMetadata() error = %v", err)
	}
	if metadata["source"] != "login" || metadata["attempts"] != float64(2) {
		t.Fatalf("DecodeMetadata() = %#v", metadata)
	}

	if _, err := DecodeMetadata([]byte(`{`)); err == nil {
		t.Fatal("DecodeMetadata() error = nil, want invalid JSON error")
	}
}
