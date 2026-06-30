package store

import (
	"log"
	"sync"
	"time"

	"obs-platform/collector/internal/chstore"
)

const maxSize = 10_000 // keep last 10k spans in memory

// StoredSpan is a richer span record that includes attributes for root-cause analysis.
type StoredSpan struct {
	TraceID     string
	SpanID      string
	ServiceName string
	IsError     bool
	Attributes  map[string]string
	ReceivedAt  time.Time
}

// Store is a thread-safe rolling buffer of recent spans.
type Store struct {
	mu    sync.RWMutex
	spans []*StoredSpan
}

func New() *Store {
	return &Store{}
}

func (s *Store) Add(spans []*StoredSpan) {
	if len(spans) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans = append(s.spans, spans...)
	if len(s.spans) > maxSize {
		s.spans = s.spans[len(s.spans)-maxSize:]
	}
}

// Rehydrate pre-populates the store with recent spans from ClickHouse.
// Must be called before the collector starts serving traffic.
func (s *Store) Rehydrate(ch *chstore.Client) {
	rows, err := ch.RecentSpans(5 * time.Minute)
	if err != nil {
		log.Printf("store: rehydrate failed: %v", err)
		return
	}
	spans := make([]*StoredSpan, 0, len(rows))
	for _, r := range rows {
		attrs := make(map[string]string, len(r.AttrKeys))
		for i, k := range r.AttrKeys {
			if i < len(r.AttrValues) {
				attrs[k] = r.AttrValues[i]
			}
		}
		spans = append(spans, &StoredSpan{
			TraceID:     r.TraceID,
			SpanID:      r.SpanID,
			ServiceName: r.ServiceName,
			IsError:     r.IsError,
			Attributes:  attrs,
			ReceivedAt:  r.ReceivedAt,
		})
	}
	s.Add(spans)
	log.Printf("store: rehydrated %d spans from ClickHouse", len(spans))
}

// SpansForService returns all stored spans for the given service name.
func (s *Store) SpansForService(name string) []*StoredSpan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*StoredSpan
	for _, sp := range s.spans {
		if sp.ServiceName == name {
			result = append(result, sp)
		}
	}
	return result
}
