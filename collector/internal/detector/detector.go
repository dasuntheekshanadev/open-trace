package detector

import (
	"log"
	"math"
	"sync"
	"time"

	"obs-platform/collector/internal/chstore"
	"obs-platform/collector/internal/graph"
)

const (
	bucketDuration = 10 * time.Second
	windowSize     = 30 // 30 buckets × 10s = 5-minute rolling baseline

	// A bucket must have at least this many calls to count toward the baseline
	// or trigger detection. Low-traffic buckets have high variance by chance alone.
	minCallsPerBucket = 5

	// Need this many valid baseline buckets before we trust the baseline enough
	// to flag anything. At 10s per bucket this is ~100s of warmup.
	minBaselineSamples = 10

	// Error rate spikes hard and fast — 2σ is the right sensitivity.
	errorRateSigma = 2.0

	// Latency varies naturally by 2-3× under normal load. Require a higher bar
	// to avoid flagging normal jitter as an incident.
	latencySigma = 3.0
)

// Anomaly represents a single detected anomaly on an edge.
type Anomaly struct {
	Source     string    `json:"source"`
	Target     string    `json:"target"`
	Metric     string    `json:"metric"`      // "error_rate" or "avg_latency_ms"
	Value      float64   `json:"value"`       // current bucket value
	Mean       float64   `json:"mean"`        // baseline mean
	StdDev     float64   `json:"stddev"`      // baseline stddev
	Threshold  float64   `json:"threshold"`   // mean + 2*stddev
	DetectedAt time.Time `json:"detected_at"`
}

// bucketSample holds per-bucket metrics for one edge. -1 means no traffic.
type bucketSample struct {
	errorRate    float64
	avgLatencyMs float64
}

type edgeWindow struct {
	samples [windowSize]bucketSample
	head    int // next write position
	count   int // slots populated (caps at windowSize)
}

type edgeCursor struct {
	lastCallCount      int64
	lastErrorCount     int64
	lastTotalLatencyMs int64
}

// Detector runs a rolling-window anomaly check against the dependency graph.
type Detector struct {
	g       *graph.Graph
	ch      *chstore.Client // nil if ClickHouse not configured
	mu      sync.RWMutex
	windows map[graph.EdgeKey]*edgeWindow
	cursors map[graph.EdgeKey]*edgeCursor
	latest  []Anomaly
}

func New(g *graph.Graph, ch *chstore.Client) *Detector {
	return &Detector{
		g:       g,
		ch:      ch,
		windows: make(map[graph.EdgeKey]*edgeWindow),
		cursors: make(map[graph.EdgeKey]*edgeCursor),
		latest:  []Anomaly{},
	}
}

// Rehydrate pre-populates the detector's rolling windows from ClickHouse history.
// Must be called before Start() so no goroutine contention exists.
func (d *Detector) Rehydrate() {
	if d.ch == nil {
		return
	}
	since := time.Duration(windowSize) * bucketDuration
	rows, err := d.ch.AllRecentEdgeMetrics(since)
	if err != nil {
		log.Printf("detector: rehydrate failed: %v", err)
		return
	}
	for _, row := range rows {
		if row.CallCount < minCallsPerBucket {
			continue
		}
		key := graph.EdgeKey{Source: row.Source, Target: row.Target}
		if d.windows[key] == nil {
			d.windows[key] = &edgeWindow{}
		}
		w := d.windows[key]
		w.samples[w.head] = bucketSample{
			errorRate:    float64(row.ErrorCount) / float64(row.CallCount),
			avgLatencyMs: float64(row.TotalLatencyMs) / float64(row.CallCount),
		}
		w.head = (w.head + 1) % windowSize
		if w.count < windowSize {
			w.count++
		}
	}
	log.Printf("detector: rehydrated %d edge metric rows from ClickHouse", len(rows))
}

func (d *Detector) Start() {
	go func() {
		ticker := time.NewTicker(bucketDuration)
		for range ticker.C {
			d.tick()
		}
	}()
}

// Anomalies returns the most recently detected anomaly set.
func (d *Detector) Anomalies() []Anomaly {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Anomaly, len(d.latest))
	copy(out, d.latest)
	return out
}

func (d *Detector) tick() {
	snapshot := d.g.Snapshot()
	now := time.Now()

	var detected []Anomaly

	for key, stats := range snapshot {
		cursor, ok := d.cursors[key]
		if !ok {
			// First time we see this edge — initialize cursor, skip this bucket.
			d.cursors[key] = &edgeCursor{
				lastCallCount:      stats.CallCount,
				lastErrorCount:     stats.ErrorCount,
				lastTotalLatencyMs: stats.TotalLatencyMs,
			}
			d.windows[key] = &edgeWindow{}
			continue
		}

		deltaCalls := stats.CallCount - cursor.lastCallCount
		deltaErrors := stats.ErrorCount - cursor.lastErrorCount
		deltaLatency := stats.TotalLatencyMs - cursor.lastTotalLatencyMs

		cursor.lastCallCount = stats.CallCount
		cursor.lastErrorCount = stats.ErrorCount
		cursor.lastTotalLatencyMs = stats.TotalLatencyMs

		var sample bucketSample
		if deltaCalls < minCallsPerBucket {
			// Too few calls — variance is meaningless at this sample size.
			sample = bucketSample{errorRate: -1, avgLatencyMs: -1}
		} else {
			sample = bucketSample{
				errorRate:    float64(deltaErrors) / float64(deltaCalls),
				avgLatencyMs: float64(deltaLatency) / float64(deltaCalls),
			}
		}

		w := d.windows[key]
		w.samples[w.head] = sample
		w.head = (w.head + 1) % windowSize
		if w.count < windowSize {
			w.count++
		}

		if sample.errorRate < 0 {
			continue // this bucket had no usable traffic
		}

		// Persist this bucket to ClickHouse for long-term baseline and restart recovery.
		if d.ch != nil {
			m := chstore.EdgeMetricRow{
				BucketTime:     now,
				Source:         key.Source,
				Target:         key.Target,
				CallCount:      deltaCalls,
				ErrorCount:     deltaErrors,
				TotalLatencyMs: deltaLatency,
			}
			go func() {
				if err := d.ch.InsertEdgeMetric(m); err != nil {
					log.Printf("detector: clickhouse insert metric: %v", err)
				}
			}()
		}

		if a := d.checkEdge(key, w, now); a != nil {
			detected = append(detected, a...)
		}
	}

	d.mu.Lock()
	if detected == nil {
		d.latest = []Anomaly{}
	} else {
		d.latest = detected
	}
	d.mu.Unlock()
}

// checkEdge runs the statistical test for both metrics on a single edge.
func (d *Detector) checkEdge(key graph.EdgeKey, w *edgeWindow, now time.Time) []Anomaly {
	// Most recent sample is at (head-1).
	recentIdx := (w.head - 1 + windowSize) % windowSize
	recent := w.samples[recentIdx]

	// Collect baseline: all samples except the most recent.
	var baselineErrors, baselineLatencies []float64
	for i := 1; i < w.count; i++ {
		idx := (w.head - 1 - i + windowSize) % windowSize
		s := w.samples[idx]
		if s.errorRate >= 0 {
			baselineErrors = append(baselineErrors, s.errorRate)
			baselineLatencies = append(baselineLatencies, s.avgLatencyMs)
		}
	}

	var anomalies []Anomaly

	if len(baselineErrors) >= minBaselineSamples {
		if a := checkMetric(key, "error_rate", recent.errorRate, baselineErrors, errorRateSigma, now); a != nil {
			anomalies = append(anomalies, *a)
		}
		if a := checkMetric(key, "avg_latency_ms", recent.avgLatencyMs, baselineLatencies, latencySigma, now); a != nil {
			anomalies = append(anomalies, *a)
		}
	}

	return anomalies
}

func checkMetric(key graph.EdgeKey, metric string, current float64, baseline []float64, sigma float64, now time.Time) *Anomaly {
	mean, stddev := meanStddev(baseline)
	threshold := mean + sigma*stddev

	anomalous := false
	if stddev == 0 {
		anomalous = current > mean
	} else {
		anomalous = current > threshold
	}

	if !anomalous {
		return nil
	}
	return &Anomaly{
		Source:     key.Source,
		Target:     key.Target,
		Metric:     metric,
		Value:      current,
		Mean:       mean,
		StdDev:     stddev,
		Threshold:  threshold,
		DetectedAt: now,
	}
}

func meanStddev(values []float64) (mean, stddev float64) {
	n := float64(len(values))
	for _, v := range values {
		mean += v
	}
	mean /= n

	var variance float64
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	variance /= n
	stddev = math.Sqrt(variance)
	return
}
