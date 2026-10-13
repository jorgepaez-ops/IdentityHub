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

	exportInput  ListInput
	exportEvents []Event
	exportErr    error
	recorded     []ExportRecord
	recordErr    error
}

func (r *auditLogRepositoryStub) ExportAuditLog(_ context.Context, input ListInput) ([]Event, error) {
	r.exportInput = input
	return r.exportEvents, r.exportErr
}

func (r *auditLogRepositoryStub) RecordAuditExport(_ context.Context, record ExportRecord) error {
	if r.recordErr != nil {
		return r.recordErr
	}
	r.recorded = append(r.recorded, record)
	return nil
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

func TestRF019_ExportAcotaLasFilasYMarcaElRecorte(t *testing.T) {
	repository := &auditLogRepositoryStub{exportEvents: make([]Event, MaxExportRows+1)}
	result, err := New(repository).Export(context.Background(), ExportInput{Format: "csv"})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if repository.exportInput.Limit != MaxExportRows+1 {
		t.Fatalf("repository limit = %d, want %d", repository.exportInput.Limit, MaxExportRows+1)
	}
	if len(result.Events) != MaxExportRows || !result.Truncated {
		t.Fatalf("events=%d truncated=%v, want %d and true", len(result.Events), result.Truncated, MaxExportRows)
	}
	if len(repository.recorded) != 1 || repository.recorded[0].Rows != MaxExportRows || !repository.recorded[0].Truncated {
		t.Fatalf("recorded = %+v, want one truncated record of %d rows", repository.recorded, MaxExportRows)
	}
}

func TestRF019_ExportRegistraLaAccionConFormatoYFiltros(t *testing.T) {
	action := "login_failed"
	repository := &auditLogRepositoryStub{exportEvents: []Event{{ID: 1}, {ID: 2}}}
	result, err := New(repository).Export(context.Background(), ExportInput{Format: "json", Action: &action})
	if err != nil || result.Truncated || len(result.Events) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if repository.exportInput.Action == nil || *repository.exportInput.Action != action {
		t.Fatalf("filters not forwarded: %+v", repository.exportInput)
	}
	if len(repository.recorded) != 1 || repository.recorded[0].Format != "json" || repository.recorded[0].Rows != 2 || repository.recorded[0].Truncated {
		t.Fatalf("recorded = %+v", repository.recorded)
	}
}

func TestRF019_ExportFallaCerradoSiNoSePuedeAuditar(t *testing.T) {
	recordErr := errors.New("audit insert failed")
	repository := &auditLogRepositoryStub{exportEvents: []Event{{ID: 1}}, recordErr: recordErr}
	result, err := New(repository).Export(context.Background(), ExportInput{Format: "csv"})
	if !errors.Is(err, recordErr) || len(result.Events) != 0 {
		t.Fatalf("result=%+v err=%v, want no data and wrapped %v", result, err, recordErr)
	}
	if _, err := New(nil).Export(context.Background(), ExportInput{}); err == nil {
		t.Fatal("Export() with nil repository must fail")
	}
	loadErr := errors.New("load failed")
	if _, err := New(&auditLogRepositoryStub{exportErr: loadErr}).Export(context.Background(), ExportInput{}); !errors.Is(err, loadErr) {
		t.Fatalf("err=%v, want wrapped %v", err, loadErr)
	}
}
