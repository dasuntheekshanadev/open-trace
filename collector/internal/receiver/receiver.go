package receiver

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	collectorv1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"obs-platform/collector/internal/chstore"
	"obs-platform/collector/internal/graph"
	"obs-platform/collector/internal/store"
)

// Receiver handles incoming OTLP HTTP trace payloads.
type Receiver struct {
	g              *graph.Graph
	st             *store.Store
	ch             *chstore.Client // nil if ClickHouse not configured
	jaegerEndpoint string
}

func New(g *graph.Graph, st *store.Store, ch *chstore.Client, jaegerEndpoint string) *Receiver {
	return &Receiver{g: g, st: st, ch: ch, jaegerEndpoint: jaegerEndpoint}
}

// Handle implements POST /v1/traces (OTLP HTTP trace endpoint).
func (r *Receiver) Handle(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Decompress gzip — otlphttp exporter sends gzip by default
	if req.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			http.Error(w, "failed to decompress body", http.StatusBadRequest)
			log.Printf("receiver: gzip decompress error: %v", err)
			return
		}
		body, err = io.ReadAll(gr)
		gr.Close()
		if err != nil {
			http.Error(w, "failed to read decompressed body", http.StatusBadRequest)
			return
		}
	}

	var exportReq collectorv1.ExportTraceServiceRequest
	if err := proto.Unmarshal(body, &exportReq); err != nil {
		http.Error(w, "failed to parse protobuf", http.StatusBadRequest)
		log.Printf("receiver: unmarshal error: %v", err)
		return
	}

	spans := extractSpans(&exportReq)
	r.g.ProcessSpans(spans)

	storedSpans := extractStoredSpans(&exportReq)
	r.st.Add(storedSpans)

	if r.ch != nil {
		go r.persistSpans(storedSpans)
	}

	// Forward the raw bytes to Jaeger so its UI still works alongside our collector.
	if r.jaegerEndpoint != "" {
		go r.forward(body)
	}

	w.WriteHeader(http.StatusOK)
}

func (r *Receiver) persistSpans(spans []*store.StoredSpan) {
	rows := make([]chstore.SpanRow, 0, len(spans))
	for _, sp := range spans {
		keys := make([]string, 0, len(sp.Attributes))
		vals := make([]string, 0, len(sp.Attributes))
		for k, v := range sp.Attributes {
			keys = append(keys, k)
			vals = append(vals, v)
		}
		rows = append(rows, chstore.SpanRow{
			ReceivedAt:  sp.ReceivedAt,
			TraceID:     sp.TraceID,
			SpanID:      sp.SpanID,
			ServiceName: sp.ServiceName,
			IsError:     sp.IsError,
			AttrKeys:    keys,
			AttrValues:  vals,
		})
	}
	if err := r.ch.InsertSpans(rows); err != nil {
		log.Printf("receiver: clickhouse insert spans: %v", err)
	}
}

func (r *Receiver) forward(body []byte) {
	resp, err := http.Post(r.jaegerEndpoint+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		log.Printf("receiver: jaeger forward error: %v", err)
		return
	}
	resp.Body.Close()
}

func extractSpans(req *collectorv1.ExportTraceServiceRequest) []*graph.Span {
	var spans []*graph.Span
	now := time.Now()

	for _, rs := range req.ResourceSpans {
		serviceName := serviceNameFromResource(rs)
		if serviceName == "" {
			continue
		}
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				spans = append(spans, &graph.Span{
					TraceID:      hex.EncodeToString(sp.TraceId),
					SpanID:       hex.EncodeToString(sp.SpanId),
					ParentSpanID: hex.EncodeToString(sp.ParentSpanId),
					ServiceName:  serviceName,
					StartTimeNs:  sp.StartTimeUnixNano,
					EndTimeNs:    sp.EndTimeUnixNano,
					IsError:      sp.Status != nil && sp.Status.Code == tracev1.Status_STATUS_CODE_ERROR,
					ReceivedAt:   now,
				})
			}
		}
	}
	return spans
}

func serviceNameFromResource(rs *tracev1.ResourceSpans) string {
	if rs.Resource == nil {
		return ""
	}
	for _, attr := range rs.Resource.Attributes {
		if attr.Key == "service.name" {
			return attr.Value.GetStringValue()
		}
	}
	return ""
}

func extractStoredSpans(req *collectorv1.ExportTraceServiceRequest) []*store.StoredSpan {
	var spans []*store.StoredSpan
	now := time.Now()

	for _, rs := range req.ResourceSpans {
		serviceName := serviceNameFromResource(rs)
		if serviceName == "" {
			continue
		}
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				attrs := make(map[string]string, len(sp.Attributes))
				for _, kv := range sp.Attributes {
					attrs[kv.Key] = anyValueToString(kv.Value)
				}
				spans = append(spans, &store.StoredSpan{
					TraceID:     hex.EncodeToString(sp.TraceId),
					SpanID:      hex.EncodeToString(sp.SpanId),
					ServiceName: serviceName,
					IsError:     sp.Status != nil && sp.Status.Code == tracev1.Status_STATUS_CODE_ERROR,
					Attributes:  attrs,
					ReceivedAt:  now,
				})
			}
		}
	}
	return spans
}

func anyValueToString(v *commonv1.AnyValue) string {
	if v == nil {
		return ""
	}
	switch x := v.Value.(type) {
	case *commonv1.AnyValue_StringValue:
		return x.StringValue
	case *commonv1.AnyValue_IntValue:
		return fmt.Sprintf("%d", x.IntValue)
	case *commonv1.AnyValue_BoolValue:
		if x.BoolValue {
			return "true"
		}
		return "false"
	case *commonv1.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", x.DoubleValue)
	default:
		return ""
	}
}
