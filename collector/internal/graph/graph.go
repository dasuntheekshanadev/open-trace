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
	mu      sync.RWMutex
	edges   map[EdgeKey]*EdgeStats
	spans   map[string]*Span   // spanID → span
	pending map[string][]*Span // parentSpanID → child spans waiting for that parent
}

func New() *Graph {
	g := &Graph{
		edges:   make(map[EdgeKey]*EdgeStats),
		spans:   make(map[string]*Span),
		pending: make(map[string][]*Span),
	}
	go g.gcLoop()
	go g.pendingGCLoop()
	return g
}

// ProcessSpans indexes the incoming spans and resolves any cross-service edges.
//
// Two-phase resolution:
//  1. Index all spans in the batch and immediately resolve any pending children
//     that were waiting for one of these parent spans.
//  2. For each span in the batch, if its parent is now in the map → record edge.
//     If parent is still missing → queue the child in pending.
func (g *Graph) ProcessSpans(spans []*Span) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Phase 1 — index and resolve pending in one pass
	for _, s := range spans {
		g.spans[s.SpanID] = s

		// Resolve child spans that arrived before this parent
		if children, ok := g.pending[s.SpanID]; ok {
			for _, child := range children {
				if s.ServiceName != child.ServiceName {
					g.recordEdge(s, child)
				}
			}
			delete(g.pending, s.SpanID)
		}
	}

	// Phase 2 — forward resolution for spans whose parent is already indexed
	for _, s := range spans {
		if s.ParentSpanID == "" {
			continue
		}
		parent, ok := g.spans[s.ParentSpanID]
		if !ok {
			// Parent not yet seen — buffer for later resolution
			g.pending[s.ParentSpanID] = append(g.pending[s.ParentSpanID], s)
			continue
		}
		if parent.ServiceName == s.ServiceName {
			continue
		}
		g.recordEdge(parent, s)
	}
}

func (g *Graph) recordEdge(parent, child *Span) {
	key := EdgeKey{Source: parent.ServiceName, Target: child.ServiceName}
	stats := g.edges[key]
	if stats == nil {
		stats = &EdgeStats{}
		g.edges[key] = stats
	}
	stats.CallCount++
	if child.IsError {
		stats.ErrorCount++
	}
	latencyMs := int64(child.EndTimeNs-child.StartTimeNs) / 1_000_000
	stats.TotalLatencyMs += latencyMs
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

// pendingGCLoop expires unresolved pending spans after 30 seconds to prevent
// unbounded growth. A span that waited 30 s for its parent will never be seen.
func (g *Graph) pendingGCLoop() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		cutoff := time.Now().Add(-30 * time.Second)
		g.mu.Lock()
		for parentID, children := range g.pending {
			fresh := children[:0]
			for _, c := range children {
				if c.ReceivedAt.After(cutoff) {
					fresh = append(fresh, c)
				}
			}
			if len(fresh) == 0 {
				delete(g.pending, parentID)
			} else {
				g.pending[parentID] = fresh
			}
		}
		g.mu.Unlock()
	}
}
