package analyzer

import (
	"sort"

	"obs-platform/collector/internal/store"
)

// Candidate is a single span attribute-value pair ranked by how correlated it is with failures.
type Candidate struct {
	Attribute               string  `json:"attribute"`
	Value                   string  `json:"value"`
	FailureRateWithValue    float64 `json:"failure_rate_with_value"`
	FailureRateWithoutValue float64 `json:"failure_rate_without_value"`
	Skew                    float64 `json:"skew"` // how much more likely to fail with this value
	SampleCount             int     `json:"sample_count"`
}

// Result is the root-cause analysis output for a single edge.
type Result struct {
	Source        string      `json:"source"`
	Target        string      `json:"target"`
	Candidates    []Candidate `json:"candidates"`
	SpansAnalyzed int         `json:"spans_analyzed"`
	FailedSpans   int         `json:"failed_spans"`
}

// responseAttributes are set as a consequence of a request failing, not as a cause.
// Including them would always rank them #1 since they correlate perfectly with errors.
var responseAttributes = map[string]bool{
	"http.status_code":              true,
	"http.response_content_length": true,
	"http.response_body_size":       true,
	"otel.status_code":              true,
	"otel.status_description":       true,
	"error":                         true,
	"exception.type":                true,
	"exception.message":             true,
	"exception.stacktrace":          true,
}

type attrKey struct{ attr, val string }

type attrStats struct {
	withCount  int
	withErrors int
}

// Analyze compares failed vs succeeded spans on the target service and surfaces
// the attribute values most correlated with failures.
func Analyze(st *store.Store, source, target string) *Result {
	return AnalyzeSpans(st.SpansForService(target), source, target)
}

// AnalyzeSpans runs the same analysis against an explicit span slice.
// Used by the API when ClickHouse provides a richer history than the in-memory store.
func AnalyzeSpans(spans []*store.StoredSpan, source, target string) *Result {
	result := &Result{
		Source:        source,
		Target:        target,
		SpansAnalyzed: len(spans),
		Candidates:    []Candidate{},
	}

	if len(spans) == 0 {
		return result
	}

	for _, sp := range spans {
		if sp.IsError {
			result.FailedSpans++
		}
	}

	// No skew possible if everything succeeded or everything failed.
	if result.FailedSpans == 0 || result.FailedSpans == len(spans) {
		return result
	}

	stats := make(map[attrKey]*attrStats)
	for _, sp := range spans {
		for k, v := range sp.Attributes {
			if responseAttributes[k] {
				continue
			}
			key := attrKey{k, v}
			s := stats[key]
			if s == nil {
				s = &attrStats{}
				stats[key] = s
			}
			s.withCount++
			if sp.IsError {
				s.withErrors++
			}
		}
	}

	total := len(spans)
	totalErrors := result.FailedSpans

	for key, s := range stats {
		if s.withCount < 3 {
			continue
		}
		withoutCount := total - s.withCount
		if withoutCount == 0 {
			continue
		}

		failureRateWith := float64(s.withErrors) / float64(s.withCount)
		failureRateWithout := float64(totalErrors-s.withErrors) / float64(withoutCount)
		skew := failureRateWith - failureRateWithout

		result.Candidates = append(result.Candidates, Candidate{
			Attribute:               key.attr,
			Value:                   key.val,
			FailureRateWithValue:    failureRateWith,
			FailureRateWithoutValue: failureRateWithout,
			Skew:                    skew,
			SampleCount:             s.withCount,
		})
	}

	sort.Slice(result.Candidates, func(i, j int) bool {
		return result.Candidates[i].Skew > result.Candidates[j].Skew
	})

	if len(result.Candidates) > 10 {
		result.Candidates = result.Candidates[:10]
	}

	return result
}
