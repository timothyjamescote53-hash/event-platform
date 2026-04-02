package repository

import (
	"context"
	"sync"
	"time"

	"github.com/yourorg/event-platform/services/ingestion/internal/service"
)

// MemoryProducer is a no-op producer for local/test use
type MemoryProducer struct {
	mu     sync.Mutex
	events []*service.Event
}

func NewMemoryProducer() service.EventProducer {
	return &MemoryProducer{}
}

func (m *MemoryProducer) Publish(_ context.Context, _ string, events []*service.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
	return nil
}

func (m *MemoryProducer) Close() error { return nil }

// MemoryDeduplicator uses an in-memory map for deduplication
type MemoryDeduplicator struct {
	mu   sync.RWMutex
	seen map[string]time.Time
}

func NewMemoryDeduplicator() service.Deduplicator {
	d := &MemoryDeduplicator{seen: make(map[string]time.Time)}
	go d.runExpiry()
	return d
}

func (m *MemoryDeduplicator) IsDuplicate(_ context.Context, id string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.seen[id]
	return exists, nil
}

func (m *MemoryDeduplicator) MarkSeen(_ context.Context, id string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[id] = time.Now()
	return nil
}

func (m *MemoryDeduplicator) runExpiry() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		cutoff := time.Now().Add(-24 * time.Hour)
		m.mu.Lock()
		for id, t := range m.seen {
			if t.Before(cutoff) {
				delete(m.seen, id)
			}
		}
		m.mu.Unlock()
	}
}
// producer
// deduper
// producer
// deduper
// producer
// deduper
// producer
// thread safe
// deduper
// expiry goroutine
// interface check
// dedup interface
// producer
// thread safe
// deduper
// expiry goroutine
// interface check
// dedup interface
// producer
