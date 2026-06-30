# Project: OpenTrace (working name) — Open Source Observability & Root-Cause Platform

## Vision
An open-source observability platform that doesn't just detect anomalies, but
helps engineers find the *actual root cause* of incidents — by building a live
service dependency graph from real traffic and statistically comparing failing
vs. healthy requests, instead of just dumping a list of "things that changed."

Long-term goal: make incident root-cause analysis accessible to small teams
who can't afford enterprise observability tools, as a genuine open-source
contribution.

## Tech stack
- Backend: Go (services + ingestion/analysis engine)
- Tracing standard: OpenTelemetry (OTLP)
- Trace storage/visualization (phase 1): Jaeger
- Frontend: React + TypeScript + D3.js (custom dependency graph visualization)
- Orchestration: Docker Compose (local dev)
- Future storage: ClickHouse or Prometheus for metrics/time-series data

## Project structure
obs-platform/
├── sample-app/              # Demo microservices that generate realistic traffic + traces
│   ├── gateway/             # Entry point, calls checkout
│   ├── checkout/            # Calls payment + inventory
│   ├── payment/             # Simulates occasional failures/latency
│   └── inventory/
├── collector/               # OUR core product: ingests traces, builds dependency graph,
│                             # runs anomaly detection + root-cause ranking
├── frontend/                # React + TS + D3 dashboard: graph viz, anomaly view, incident view
├── docker-compose.yml       # Wires everything together (sample-app + Jaeger + collector + frontend)
└── docs/
    └── architecture.md      # Living architecture doc, updated as we build

## Build phases (build in this order — don't skip ahead)

### Phase 1 — Generate real trace data
Build the 4 sample-app Go services. Each one:
- Exposes a simple HTTP endpoint
- Calls the next service in the chain (gateway -> checkout -> payment + inventory)
- Instrumented with OpenTelemetry SDK, exporting traces via OTLP to Jaeger
- Payment service randomly fails ~5% of requests and randomly adds latency spikes,
  to simulate real-world flakiness
- A simple load generator script hits the gateway continuously

Success criteria: traces visible in Jaeger UI, showing the full call chain,
including the random failures.

### Phase 2 — Build our own dependency graph from trace data
In `collector/`, build a Go service that:
- Receives OTLP trace data (instead of, or alongside, Jaeger)
- Parses spans: trace_id, span_id, parent_span_id, service name, start/end time, status
- Builds an in-memory graph: nodes = services, edges = "calls" relationships
- Tracks per-edge stats: call volume, error rate, average latency (rolling window)
- Exposes this graph via a simple REST/JSON API (e.g., GET /graph)

Success criteria: hitting /graph returns a JSON representation of the live
service map with real traffic stats per edge.

### Phase 3 — Anomaly detection
Add to the collector:
- For each edge in the graph, maintain a rolling average + standard deviation
  of error rate and latency
- Flag when live values exceed N standard deviations from baseline
- Expose flagged anomalies via API (e.g., GET /anomalies)

Success criteria: artificially crank up the payment service's failure rate,
watch the anomaly get flagged within the rolling window.

### Phase 4 — Root cause ranking (the hard, interesting part)
Add to the collector:
- When an anomaly fires on an edge, pull recent traces involving that edge
- Split traces into "failed" vs "succeeded" for that edge
- Compare attributes across the two groups (which downstream call, which pod,
  which version tag, etc. — whatever attributes are attached to spans)
- Surface the attribute(s) with the most skewed distribution between groups
  as the likely root cause signal

Success criteria: inject a fault tied to a specific attribute (e.g., only pod
"payment-2" fails), and the tool correctly surfaces that attribute as the
top suspect.

### Phase 5 — Frontend
React + TypeScript + D3:
- Dependency graph view (D3 force-directed graph, nodes = services, edges
  colored/sized by error rate or latency)
- Anomaly feed (list of currently flagged anomalies)
- Incident detail view (when you click an anomaly, show the root-cause
  ranking results from Phase 4)

Success criteria: visually see the service graph, see an edge turn red when
an anomaly fires, click it, and see ranked root-cause candidates.

## Design principles to follow throughout
- Build from real trace data, not architecture diagrams — the graph must be
  derived live, not hand-configured
- Favor simple statistical methods (rolling mean + std dev, distribution
  comparison) over premature ML — these are explainable and good enough for v1
- Every phase should be independently testable end-to-end before moving to
  the next phase
- Code should be clean enough that external contributors can understand it —
  this is meant to be open source eventually

## What NOT to build yet
- No machine learning models in v1
- No multi-cloud support yet — single local Docker Compose setup only
- No authentication/multi-tenancy — single-user local tool for now
- No persistent long-term storage yet — in-memory/rolling window is fine for v1

## Current status
Not yet started. This file is the starting brief for scaffolding the project.
