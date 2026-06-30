# OpenTrace

An open-source observability platform that automatically maps how your microservices talk to each other, detects anomalies in real time, and performs root-cause analysis on failures — all from a single dashboard.

![Dashboard showing service graph with anomaly detected](docs/screenshot.png)

## What it does

- **Service graph** — automatically discovers every service-to-service connection from live traffic, no manual config
- **Anomaly detection** — rolling statistical baseline per edge; flags error rate or latency spikes within 10 seconds
- **Root-cause analysis** — compares span attributes between failed and succeeded requests to surface the discriminating factor (e.g. `payment.pod=payment-2` has a 94% failure rate vs 0% for all other pods)
- **Time-series charts** — 30-day history of error rate and latency per edge, stored in ClickHouse
- **Works with any app** — uses OpenTelemetry, the open standard; SDKs exist for every major language

## Quick start

```bash
git clone https://github.com/dasuntheekshanadev/open-trace.git
cd open-trace
docker compose up
```

Open **http://localhost:3000**

The demo sample app (4 microservices: gateway → checkout → payment + inventory) starts automatically and begins generating traffic. Within 30 seconds you'll see the service graph populate.

To simulate a failure:

```bash
# Take down one payment pod (triggers anomaly detection)
curl -X POST http://localhost:8082/admin/fault -d '{"mode":"pod-2-down"}'

# Reset
curl -X POST http://localhost:8082/admin/fault -d '{"mode":"none"}'
```

## Architecture

```
Your App  ──OTLP──►  Collector  ──►  ClickHouse (30-day storage)
                         │
                         ├──►  Anomaly Detector (rolling window, 10s buckets)
                         ├──►  Root-Cause Analyzer (span attribute skew)
                         └──►  API  ──►  Frontend Dashboard
                         │
                         └──►  Jaeger (full trace viewer, port 16686)
```

| Component | What it is |
|-----------|------------|
| `collector/` | Go service — receives OTLP traces, builds the graph, runs detection |
| `frontend/` | React + D3.js dashboard |
| `sample-app/` | Demo microservices (gateway, checkout, payment, inventory, loadgen) |
| ClickHouse | Column-store DB for span and metric history |
| Jaeger | Full distributed trace viewer (optional, runs alongside) |

## Integrating with your own app

See **[docs/integration.md](docs/integration.md)** for step-by-step instructions for Node.js, Python, Go, and Java.

The short version: install the OpenTelemetry SDK for your language, point it at `http://your-server:4319`, set a service name. Everything else is automatic.

## How anomaly detection works

For each service-to-service edge, the detector maintains a rolling window of 30 buckets (each 10 seconds = 5 minutes of baseline). Every tick it computes the delta calls, errors, and latency for that bucket and compares against the baseline mean and standard deviation.

- Error rate anomaly: current bucket > mean + 2σ
- Latency anomaly: current bucket > mean + 3σ

Buckets with fewer than 5 calls are skipped to avoid high-variance noise from low-traffic edges.

## Requirements

- Docker and Docker Compose
- Ports: `3000` (dashboard), `4319` (OTLP receiver), `16686` (Jaeger UI, optional)

## License

MIT
