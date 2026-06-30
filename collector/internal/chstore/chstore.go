// Package chstore wraps ClickHouse's HTTP interface.
// It uses only the standard library — no external driver needed.
package chstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SpanRow is the ClickHouse representation of a stored span.
type SpanRow struct {
	ReceivedAt  time.Time
	TraceID     string
	SpanID      string
	ServiceName string
	IsError     bool
	AttrKeys    []string
	AttrValues  []string
}

// EdgeMetricRow is one 10-second bucket of stats for a single edge.
type EdgeMetricRow struct {
	BucketTime     time.Time
	Source         string
	Target         string
	CallCount      int64
	ErrorCount     int64
	TotalLatencyMs int64
}

// Client talks to ClickHouse over HTTP.
type Client struct {
	addr string
	http *http.Client
}

func New(addr string) *Client {
	return &Client{
		addr: addr,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// Ping checks if ClickHouse is reachable.
func (c *Client) Ping() error {
	resp, err := c.http.Get(c.addr + "/ping")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("clickhouse ping: status %d", resp.StatusCode)
	}
	return nil
}

// WaitReady blocks until ClickHouse responds or timeout is reached.
func (c *Client) WaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := c.Ping(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("clickhouse not ready after %s", timeout)
}

// Init creates the tables if they don't exist.
func (c *Client) Init() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS spans (
			received_at  DateTime,
			trace_id     String,
			span_id      String,
			service_name String,
			is_error     UInt8,
			attr_keys    Array(String),
			attr_values  Array(String)
		) ENGINE = MergeTree()
		PARTITION BY toYYYYMMDD(received_at)
		ORDER BY (service_name, received_at)
		TTL received_at + INTERVAL 7 DAY`,

		`CREATE TABLE IF NOT EXISTS edge_metrics (
			bucket_time      DateTime,
			source           String,
			target           String,
			call_count       Int64,
			error_count      Int64,
			total_latency_ms Int64
		) ENGINE = MergeTree()
		PARTITION BY toYYYYMMDD(bucket_time)
		ORDER BY (source, target, bucket_time)
		TTL bucket_time + INTERVAL 30 DAY`,
	}
	for _, sql := range tables {
		if err := c.exec(sql); err != nil {
			return err
		}
	}
	return nil
}

// InsertSpans batch-inserts span rows using JSONEachRow format.
func (c *Client) InsertSpans(rows []SpanRow) error {
	if len(rows) == 0 {
		return nil
	}
	type row struct {
		ReceivedAt  string   `json:"received_at"`
		TraceID     string   `json:"trace_id"`
		SpanID      string   `json:"span_id"`
		ServiceName string   `json:"service_name"`
		IsError     uint8    `json:"is_error"`
		AttrKeys    []string `json:"attr_keys"`
		AttrValues  []string `json:"attr_values"`
	}
	var buf bytes.Buffer
	buf.WriteString("INSERT INTO spans FORMAT JSONEachRow\n")
	enc := json.NewEncoder(&buf)
	for _, r := range rows {
		e := uint8(0)
		if r.IsError {
			e = 1
		}
		keys := r.AttrKeys
		vals := r.AttrValues
		if keys == nil {
			keys = []string{}
		}
		if vals == nil {
			vals = []string{}
		}
		enc.Encode(row{
			ReceivedAt:  r.ReceivedAt.UTC().Format("2006-01-02 15:04:05"),
			TraceID:     r.TraceID,
			SpanID:      r.SpanID,
			ServiceName: r.ServiceName,
			IsError:     e,
			AttrKeys:    keys,
			AttrValues:  vals,
		})
	}
	return c.exec(buf.String())
}

// InsertEdgeMetric inserts a single edge bucket metric.
func (c *Client) InsertEdgeMetric(m EdgeMetricRow) error {
	type row struct {
		BucketTime     string `json:"bucket_time"`
		Source         string `json:"source"`
		Target         string `json:"target"`
		CallCount      int64  `json:"call_count"`
		ErrorCount     int64  `json:"error_count"`
		TotalLatencyMs int64  `json:"total_latency_ms"`
	}
	var buf bytes.Buffer
	buf.WriteString("INSERT INTO edge_metrics FORMAT JSONEachRow\n")
	json.NewEncoder(&buf).Encode(row{
		BucketTime:     m.BucketTime.UTC().Format("2006-01-02 15:04:05"),
		Source:         m.Source,
		Target:         m.Target,
		CallCount:      m.CallCount,
		ErrorCount:     m.ErrorCount,
		TotalLatencyMs: m.TotalLatencyMs,
	})
	return c.exec(buf.String())
}

// SpansForService returns recent spans for a given service for root-cause analysis.
func (c *Client) SpansForService(serviceName string, since time.Duration) ([]SpanRow, error) {
	sql := fmt.Sprintf(
		`SELECT received_at, trace_id, span_id, service_name, is_error, attr_keys, attr_values
		 FROM spans
		 WHERE service_name = '%s'
		 AND received_at >= now() - INTERVAL %d SECOND
		 ORDER BY received_at DESC
		 LIMIT 5000
		 FORMAT JSONEachRow`,
		escape(serviceName), int(since.Seconds()),
	)
	return c.querySpans(sql)
}

// RecentSpans returns spans for all services — used to rehydrate the in-memory store on startup.
func (c *Client) RecentSpans(since time.Duration) ([]SpanRow, error) {
	sql := fmt.Sprintf(
		`SELECT received_at, trace_id, span_id, service_name, is_error, attr_keys, attr_values
		 FROM spans
		 WHERE received_at >= now() - INTERVAL %d SECOND
		 ORDER BY received_at ASC
		 LIMIT 10000
		 FORMAT JSONEachRow`,
		int(since.Seconds()),
	)
	return c.querySpans(sql)
}

// EdgeMetricsForEdge returns bucketed metrics for a specific source→target edge.
func (c *Client) EdgeMetricsForEdge(source, target string, since time.Duration) ([]EdgeMetricRow, error) {
	sql := fmt.Sprintf(
		`SELECT bucket_time, source, target, call_count, error_count, total_latency_ms
		 FROM edge_metrics
		 WHERE source = '%s' AND target = '%s'
		 AND bucket_time >= now() - INTERVAL %d SECOND
		 ORDER BY bucket_time ASC
		 FORMAT JSONEachRow`,
		escape(source), escape(target), int(since.Seconds()),
	)
	return c.queryEdgeMetrics(sql)
}

// AllRecentEdgeMetrics returns recent edge bucket metrics — used to rehydrate the detector on startup.
func (c *Client) AllRecentEdgeMetrics(since time.Duration) ([]EdgeMetricRow, error) {
	sql := fmt.Sprintf(
		`SELECT bucket_time, source, target, call_count, error_count, total_latency_ms
		 FROM edge_metrics
		 WHERE bucket_time >= now() - INTERVAL %d SECOND
		 ORDER BY bucket_time ASC
		 FORMAT JSONEachRow`,
		int(since.Seconds()),
	)
	return c.queryEdgeMetrics(sql)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (c *Client) exec(sql string) error {
	resp, err := c.http.Post(c.addr, "text/plain", strings.NewReader(sql))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clickhouse: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Client) query(sql string) (io.ReadCloser, error) {
	resp, err := c.http.Post(c.addr, "text/plain", strings.NewReader(sql))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("clickhouse query: %s", strings.TrimSpace(string(body)))
	}
	return resp.Body, nil
}

type spanJSON struct {
	ReceivedAt  string   `json:"received_at"`
	TraceID     string   `json:"trace_id"`
	SpanID      string   `json:"span_id"`
	ServiceName string   `json:"service_name"`
	IsError     uint8    `json:"is_error"`
	AttrKeys    []string `json:"attr_keys"`
	AttrValues  []string `json:"attr_values"`
}

func (c *Client) querySpans(sql string) ([]SpanRow, error) {
	body, err := c.query(sql)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var rows []SpanRow
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB per row
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r spanJSON
		if err := json.Unmarshal(line, &r); err != nil {
			log.Printf("chstore: parse span row: %v", err)
			continue
		}
		t, _ := time.Parse("2006-01-02 15:04:05", r.ReceivedAt)
		rows = append(rows, SpanRow{
			ReceivedAt:  t,
			TraceID:     r.TraceID,
			SpanID:      r.SpanID,
			ServiceName: r.ServiceName,
			IsError:     r.IsError == 1,
			AttrKeys:    r.AttrKeys,
			AttrValues:  r.AttrValues,
		})
	}
	return rows, scanner.Err()
}

// ClickHouse returns Int64 columns as JSON strings to preserve precision,
// so we parse them as strings then convert.
type edgeMetricJSON struct {
	BucketTime     string `json:"bucket_time"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	CallCount      string `json:"call_count"`
	ErrorCount     string `json:"error_count"`
	TotalLatencyMs string `json:"total_latency_ms"`
}

func (c *Client) queryEdgeMetrics(sql string) ([]EdgeMetricRow, error) {
	body, err := c.query(sql)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var rows []EdgeMetricRow
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r edgeMetricJSON
		if err := json.Unmarshal(line, &r); err != nil {
			log.Printf("chstore: parse edge metric row: %v", err)
			continue
		}
		t, _ := time.Parse("2006-01-02 15:04:05", r.BucketTime)
		rows = append(rows, EdgeMetricRow{
			BucketTime:     t,
			Source:         r.Source,
			Target:         r.Target,
			CallCount:      parseInt64(r.CallCount),
			ErrorCount:     parseInt64(r.ErrorCount),
			TotalLatencyMs: parseInt64(r.TotalLatencyMs),
		})
	}
	return rows, scanner.Err()
}

func escape(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
