package graph

import (
	"sync"
	"time"
)

// Span holds the fields we care about from an incoming OTLP span.
type Span struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	ServiceName  string
	StartTimeNs  uint64
	EndTimeNs    uint64
	IsError      bool
	ReceivedAt   time.Time
}

// EdgeKey uniquely identifies a directed call relationship between two services.
type EdgeKey struct {
	Source string // the service that made the call
	Target string // the service that received the call
}

// EdgeStats accumulates traffic stats for a single edge.
type EdgeStats struct {
	CallCount      int64
	ErrorCount     int64
	TotalLatencyMs int64
}

// Graph is a thread-safe, in-memory service dependency graph.
type Graph struct {
	mu    sync.RWMutex
	edges map[EdgeKey]*EdgeStats
	spans map[string]*Span // spanID → span, used to resolve parent→child relationships
}

func New() *Graph {
	g := &Graph{
		edges: make(map[EdgeKey]*EdgeStats),
		spans: make(map[string]*Span),
	}
	go g.gcLoop()
	return g
}

// ProcessSpans indexes the incoming spans and resolves any cross-service edges.
// The logic: if span S has a parent span P from a different service, then
// P.ServiceName → S.ServiceName is a call edge in the graph.
func (g *Graph) ProcessSpans(spans []*Span) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Index first so parent lookups work even within the same batch.
	for _, s := range spans {
		g.spans[s.SpanID] = s
	}

	for _, s := range spans {
		if s.ParentSpanID == "" {
			continue
		}
		parent, ok := g.spans[s.ParentSpanID]
		if !ok || parent.ServiceName == s.ServiceName {
			continue
		}

		key := EdgeKey{Source: parent.ServiceName, Target: s.ServiceName}
		stats := g.edges[key]
		if stats == nil {
			stats = &EdgeStats{}
			g.edges[key] = stats
		}
		stats.CallCount++
		if s.IsError {
			stats.ErrorCount++
		}
		latencyMs := int64(s.EndTimeNs-s.StartTimeNs) / 1_000_000
		stats.TotalLatencyMs += latencyMs
	}
}

// Snapshot returns a deep copy of the current edge stats.
func (g *Graph) Snapshot() map[EdgeKey]*EdgeStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	out := make(map[EdgeKey]*EdgeStats, len(g.edges))
	for k, v := range g.edges {
		cp := *v
		out[k] = &cp
	}
	return out
}

// gcLoop removes spans older than 10 minutes from the buffer every 5 minutes.
// Without this, the span map grows forever since spans are only added, never removed.
func (g *Graph) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		g.mu.Lock()
		for id, s := range g.spans {
			if s.ReceivedAt.Before(cutoff) {
				delete(g.spans, id)
			}
		}
		g.mu.Unlock()
	}
}
