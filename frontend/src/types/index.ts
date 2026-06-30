export interface Edge {
  source: string;
  target: string;
  call_count: number;
  error_count: number;
  error_rate: number;
  avg_latency_ms: number;
}

export interface GraphData {
  edges: Edge[];
}

export interface Anomaly {
  source: string;
  target: string;
  metric: string;
  value: number;
  mean: number;
  stddev: number;
  threshold: number;
  detected_at: string;
}

export interface AnomalyData {
  anomalies: Anomaly[];
}

export interface Candidate {
  attribute: string;
  value: string;
  failure_rate_with_value: number;
  failure_rate_without_value: number;
  skew: number;
  sample_count: number;
}

export interface BucketPoint {
  t: number;    // unix seconds
  er: number;   // error rate 0–1
  lat: number;  // avg latency ms
}

export interface TimeSeriesData {
  source: string;
  target: string;
  buckets: BucketPoint[];
}

export interface RootCauseData {
  source: string;
  target: string;
  candidates: Candidate[];
  spans_analyzed: number;
  failed_spans: number;
}
