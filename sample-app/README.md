# Sample App — Trace Generator for OpenTrace

A tiny 4-service Go app (gateway → checkout → payment + inventory),
instrumented with OpenTelemetry, that generates realistic distributed traces
— including injectable failures — for the obs-platform to detect and analyze.

## What it does

```
loadgen --> gateway --> checkout --> payment   (random failures + latency)
                              \--> inventory  (mostly healthy)
```

All traces export via OTLP/HTTP to Jaeger (for now — later, to our own
collector once we build it).

## Run it locally

Requires Docker + Docker Compose installed.

```bash
cd sample-app
go mod tidy        # downloads dependencies and generates go.sum
docker compose up --build
```

Then open the Jaeger UI: **http://localhost:16686**
- Select service: `gateway`
- Click "Find Traces"
- You'll see live traces flowing through gateway -> checkout -> payment/inventory

## Inject a fault (to test detection later)

The payment service has an admin endpoint to switch failure modes on demand:

```bash
# Make pod "payment-2" fail almost every request (good for testing root-cause ranking)
curl -X POST localhost:8082/admin/fault -d "pod-2-down"

# Make everything slow (good for testing latency anomaly detection)
curl -X POST localhost:8082/admin/fault -d "global-slow"

# Elevate error rate globally (no specific attribute to blame — harder case)
curl -X POST localhost:8082/admin/fault -d "global-fail"

# Back to normal baseline (~2% error rate)
curl -X POST localhost:8082/admin/fault -d "none"
```

Watch the effect live in Jaeger, or later, in our own collector's /graph and
/anomalies endpoints once Phase 2/3 are built.

## Run without Docker (optional, faster iteration)

You can also run each service directly with `go run`, in separate terminals:

```bash
# Terminal 1: still need Jaeger running for OTLP ingestion + viewing
docker run -d --name jaeger -p 16686:16686 -p 4318:4318 \
  -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:1.57

# Terminal 2
go run ./cmd/inventory

# Terminal 3
go run ./cmd/payment

# Terminal 4
go run ./cmd/checkout

# Terminal 5
go run ./cmd/gateway

# Terminal 6
go run ./cmd/loadgen
```

(Default ports/URLs already point at localhost, matching the env var
fallbacks in each main.go — no extra config needed for this mode.)

## Stopping everything (Docker mode)

```bash
docker compose down
```
