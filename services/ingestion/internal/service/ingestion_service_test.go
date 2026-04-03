package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- Mocks (no external deps) ---

type mockProducer struct {
	published int
}

func (m *mockProducer) Publish(_ context.Context, _ string, events []*Event) error {
	m.published += len(events)
	return nil
}
func (m *mockProducer) Close() error { return nil }

type mockDeduper struct {
	seen map[string]bool
}

func newMockDeduper() *mockDeduper {
	return &mockDeduper{seen: make(map[string]bool)}
}
func (m *mockDeduper) IsDuplicate(_ context.Context, id string) (bool, error) {
	return m.seen[id], nil
}
func (m *mockDeduper) MarkSeen(_ context.Context, id string, _ time.Duration) error {
	m.seen[id] = true
	return nil
}

func newTestSvc() (*IngestionService, *mockProducer) {
	p := &mockProducer{}
	svc := NewIngestionService(p, newMockDeduper(), nil, Config{
		BatchSize:    10,
		BatchFlushMs: 50,
	})
	return svc, p
}

// --- Tests ---

func TestIngestEvent_Valid(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.IngestEvent(context.Background(), &Event{
		TenantID:  "acme",
		EventType: "purchase",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIngestEvent_AssignsID(t *testing.T) {
	svc, _ := newTestSvc()
	e := &Event{TenantID: "acme", EventType: "click"}
	svc.IngestEvent(context.Background(), e)
	if e.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestIngestEvent_AssignsTimestamp(t *testing.T) {
	svc, _ := newTestSvc()
	e := &Event{TenantID: "acme", EventType: "click"}
	svc.IngestEvent(context.Background(), e)
	if e.Timestamp == 0 {
		t.Error("expected Timestamp to be assigned")
	}
}

func TestIngestEvent_AssignsVersion(t *testing.T) {
	svc, _ := newTestSvc()
	e := &Event{TenantID: "acme", EventType: "click"}
	svc.IngestEvent(context.Background(), e)
	if e.Version != 1 {
		t.Errorf("expected Version=1, got %d", e.Version)
	}
}

func TestIngestEvent_MissingTenant(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.IngestEvent(context.Background(), &Event{EventType: "purchase"})
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}
}

func TestIngestEvent_MissingEventType(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.IngestEvent(context.Background(), &Event{TenantID: "acme"})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestIngestEvent_Duplicate(t *testing.T) {
	svc, _ := newTestSvc()
	e := &Event{ID: "dup-123", TenantID: "acme", EventType: "click"}
	svc.IngestEvent(context.Background(), e)
	err := svc.IngestEvent(context.Background(), e)
	if !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("expected ErrDuplicateEvent, got %v", err)
	}
}

func TestIngestBatch_AcceptsValid(t *testing.T) {
	svc, _ := newTestSvc()
	events := []*Event{
		{TenantID: "acme", EventType: "purchase"},
		{TenantID: "acme", EventType: "click"},
	}
	accepted, rejected, _ := svc.IngestBatch(context.Background(), events)
	if accepted != 2 {
		t.Errorf("expected 2 accepted, got %d", accepted)
	}
	if rejected != 0 {
		t.Errorf("expected 0 rejected, got %d", rejected)
	}
}

func TestIngestBatch_RejectsInvalid(t *testing.T) {
	svc, _ := newTestSvc()
	events := []*Event{
		{TenantID: "acme", EventType: "purchase"}, // valid
		{EventType: "no-tenant"},                   // invalid
		{TenantID: "acme"},                         // invalid
	}
	accepted, rejected, _ := svc.IngestBatch(context.Background(), events)
	if accepted != 1 {
		t.Errorf("expected 1 accepted, got %d", accepted)
	}
	if rejected != 2 {
		t.Errorf("expected 2 rejected, got %d", rejected)
	}
}

func TestFlush_PublishesToProducer(t *testing.T) {
	svc, producer := newTestSvc()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		svc.IngestEvent(ctx, &Event{TenantID: "acme", EventType: "click"})
	}
	svc.Flush(ctx)
	if producer.published != 3 {
		t.Errorf("expected 3 published, got %d", producer.published)
	}
}

func TestIngestEvent_NilPayload(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.IngestEvent(context.Background(), &Event{
		TenantID:  "acme",
		EventType: "ping",
		Payload:   nil,
	})
	if err != nil {
		t.Errorf("nil payload should be valid, got: %v", err)
	}
}
// valid
// invalid
// dedup
// batch
// flush
// valid
// invalid
// dedup
// batch
// flush
// valid
// invalid
// dedup
// batch
// flush
// valid test
// id assign
// timestamp
// version
// no tenant
// no type
// dedup
// batch valid
// batch mixed
// flush
// nil payload
// valid test
// id assign
// timestamp
// version
// no tenant
// no type
// dedup
// batch valid
// batch mixed
// flush
// nil payload
// valid test
// id assign
// timestamp
// version
// no tenant
// no type
// dedup
// batch valid
// batch mixed
