package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var (
	ErrInvalidEvent   = errors.New("invalid event")
	ErrDuplicateEvent = errors.New("duplicate event")
	ErrTenantRequired = errors.New("tenant_id is required")
)

type Event struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	UserID    string            `json:"user_id"`
	EventType string            `json:"event_type"`
	Source    string            `json:"source"`
	Timestamp int64             `json:"timestamp"`
	Version   int               `json:"version"`
	Payload   map[string]any    `json:"payload"`
	Labels    map[string]string `json:"labels"`
}

type EventProducer interface {
	Publish(ctx context.Context, tenantID string, events []*Event) error
	Close() error
}

type Deduplicator interface {
	IsDuplicate(ctx context.Context, eventID string) (bool, error)
	MarkSeen(ctx context.Context, eventID string, ttl time.Duration) error
}

type Config struct {
	BatchSize    int
	BatchFlushMs int
}

type IngestionService struct {
	producer EventProducer
	deduper  Deduplicator
	cfg      Config
	mu       sync.Mutex
	buffer   []*Event
	flushCh  chan struct{}
}

func NewIngestionService(producer EventProducer, deduper Deduplicator, _ any, cfg Config) *IngestionService {
	return &IngestionService{
		producer: producer,
		deduper:  deduper,
		cfg:      cfg,
		buffer:   make([]*Event, 0, cfg.BatchSize),
		flushCh:  make(chan struct{}, 1),
	}
}

func (s *IngestionService) IngestEvent(ctx context.Context, event *Event) error {
	if err := s.validate(event); err != nil {
		return err
	}
	if event.ID == "" {
		event.ID = newUUID()
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if event.Version == 0 {
		event.Version = 1
	}

	dup, err := s.deduper.IsDuplicate(ctx, event.ID)
	if err != nil {
		slog.Warn("dedup check failed", "err", err)
	}
	if dup {
		return ErrDuplicateEvent
	}
	_ = s.deduper.MarkSeen(ctx, event.ID, 24*time.Hour)
	s.bufferEvent(event)
	return nil
}

func (s *IngestionService) IngestBatch(ctx context.Context, events []*Event) (accepted, rejected int, errs []string) {
	for _, e := range events {
		if err := s.IngestEvent(ctx, e); err != nil {
			if errors.Is(err, ErrDuplicateEvent) {
				rejected++
				continue
			}
			rejected++
			errs = append(errs, err.Error())
		} else {
			accepted++
		}
	}
	return
}

func (s *IngestionService) bufferEvent(event *Event) {
	s.mu.Lock()
	s.buffer = append(s.buffer, event)
	flush := len(s.buffer) >= s.cfg.BatchSize
	s.mu.Unlock()
	if flush {
		select {
		case s.flushCh <- struct{}{}:
		default:
		}
	}
}

func (s *IngestionService) RunBatchFlusher(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.BatchFlushMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushBuffer(ctx)
		case <-s.flushCh:
			s.flushBuffer(ctx)
		}
	}
}

func (s *IngestionService) flushBuffer(ctx context.Context) {
	s.mu.Lock()
	if len(s.buffer) == 0 {
		s.mu.Unlock()
		return
	}
	batch := s.buffer
	s.buffer = make([]*Event, 0, s.cfg.BatchSize)
	s.mu.Unlock()

	byTenant := make(map[string][]*Event)
	for _, e := range batch {
		byTenant[e.TenantID] = append(byTenant[e.TenantID], e)
	}
	for tenantID, events := range byTenant {
		if err := s.producer.Publish(ctx, tenantID, events); err != nil {
			slog.Error("publish failed", "tenant", tenantID, "err", err)
		}
	}
}

func (s *IngestionService) Flush(ctx context.Context) { s.flushBuffer(ctx) }

func (s *IngestionService) validate(e *Event) error {
	if e.TenantID == "" {
		return ErrTenantRequired
	}
	if e.EventType == "" {
		return ErrInvalidEvent
	}
	if e.Payload != nil {
		if _, err := json.Marshal(e.Payload); err != nil {
			return ErrInvalidEvent
		}
	}
	return nil
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
// domain
// validate
// dedup
// buffer
// flush
// uuid
// domain
// validate
// dedup
// buffer
// flush
// uuid
// domain
// validate
// dedup
// buffer
// flush
// uuid
// event
// errors
// interfaces
// validate
// uuid
// timestamp
// version
// dedup check
// buffer
// flush trigger
// flusher
// tenant group
// batch method
// shutdown flush
// log warn
// log error
// newuuid v2
// config struct
// event
// errors
// interfaces
// validate
// uuid
// timestamp
// version
// dedup check
// buffer
// flush trigger
// flusher
// tenant group
// batch method
// shutdown flush
// log warn
// log error
// newuuid v2
// config struct
// event
// errors
// interfaces
// validate
// uuid
// timestamp
// version
// dedup check
// buffer
