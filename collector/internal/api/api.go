package api

import (
	"encoding/json"
	"net/http"
	"time"

	"obs-platform/collector/internal/analyzer"
	"obs-platform/collector/internal/chstore"
	"obs-platform/collector/internal/detector"
	"obs-platform/collector/internal/graph"
	"obs-platform/collector/internal/store"
)

type API struct {
	g  *graph.Graph
	d  *detector.Detector
	st *store.Store
	ch *chstore.Client // nil if ClickHouse not configured
}

func New(g *graph.Graph, d *detector.Detector, st *store.Store, ch *chstore.Client) *API {
	return &API{g: g, d: d, st: st, ch: ch}
}

type edgeResponse struct {
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	CallCount    int64   `json:"call_count"`
	ErrorCount   int64   `json:"error_count"`
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

type graphResponse struct {
	Edges []edgeResponse `json:"edges"`
}

// HandleRootCause serves GET /rootcause?source=X&target=Y — ranks span attributes
// by correlation with failures on the given edge.
// When ClickHouse is available it queries 30 minutes of history for better signal.
func (a *API) HandleRootCause(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	target := r.URL.Query().Get("target")
	if source == "" || target == "" {
		http.Error(w, "source and target query params required", http.StatusBadRequest)
		return
	}

	var result *analyzer.Result
	if a.ch != nil {
		rows, err := a.ch.SpansForService(target, 30*time.Minute)
		if err == nil && len(rows) > 0 {
			spans := make([]*store.StoredSpan, 0, len(rows))
			for _, row := range rows {
				attrs := make(map[string]string, len(row.AttrKeys))
				for i, k := range row.AttrKeys {
					if i < len(row.AttrValues) {
						attrs[k] = row.AttrValues[i]
					}
				}
				spans = append(spans, &store.StoredSpan{
					TraceID:     row.TraceID,
					SpanID:      row.SpanID,
					ServiceName: row.ServiceName,
					IsError:     row.IsError,
					Attributes:  attrs,
					ReceivedAt:  row.ReceivedAt,
				})
			}
			result = analyzer.AnalyzeSpans(spans, source, target)
		}
	}

	// Fall back to in-memory store if ClickHouse is not configured or returned nothing.
	if result == nil {
		result = analyzer.Analyze(a.st, source, target)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(result)
}

// HandleAnomalies serves GET /anomalies — returns currently detected anomalies.
func (a *API) HandleAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies := a.d.Anomalies()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]any{"anomalies": anomalies})
}

type bucketPoint struct {
	T   int64   `json:"t"`   // unix seconds
	Er  float64 `json:"er"`  // error rate 0–1
	Lat float64 `json:"lat"` // avg latency ms
}

type timeSeriesResp struct {
	Source  string        `json:"source"`
	Target  string        `json:"target"`
	Buckets []bucketPoint `json:"buckets"`
}

// HandleTimeSeries serves GET /timeseries?source=X&target=Y — returns bucketed
// error rate and latency over the last 30 minutes for a single edge.
func (a *API) HandleTimeSeries(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	target := r.URL.Query().Get("target")
	if source == "" || target == "" {
		http.Error(w, "source and target required", http.StatusBadRequest)
		return
	}

	buckets := []bucketPoint{}

	if a.ch != nil {
		rows, err := a.ch.EdgeMetricsForEdge(source, target, 30*time.Minute)
		if err == nil {
			for _, row := range rows {
				if row.CallCount == 0 {
					continue
				}
				buckets = append(buckets, bucketPoint{
					T:   row.BucketTime.Unix(),
					Er:  float64(row.ErrorCount) / float64(row.CallCount),
					Lat: float64(row.TotalLatencyMs) / float64(row.CallCount),
				})
			}
		}
	}

	resp := timeSeriesResp{Source: source, Target: target, Buckets: buckets}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}

// HandleGraph serves GET /graph — returns the live service dependency graph as JSON.
func (a *API) HandleGraph(w http.ResponseWriter, r *http.Request) {
	snapshot := a.g.Snapshot()

	resp := graphResponse{Edges: []edgeResponse{}}
	for key, stats := range snapshot {
		edge := edgeResponse{
			Source:     key.Source,
			Target:     key.Target,
			CallCount:  stats.CallCount,
			ErrorCount: stats.ErrorCount,
		}
		if stats.CallCount > 0 {
			edge.ErrorRate = float64(stats.ErrorCount) / float64(stats.CallCount)
			edge.AvgLatencyMs = float64(stats.TotalLatencyMs) / float64(stats.CallCount)
		}
		resp.Edges = append(resp.Edges, edge)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}
