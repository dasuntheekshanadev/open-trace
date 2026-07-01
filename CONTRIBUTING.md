# Contributing to OpenTrace

Thanks for your interest in contributing. OpenTrace is a small, focused project — contributions that keep it simple and practical are most welcome.

## What we're looking for

- Bug fixes
- Better anomaly detection algorithms
- New language integration guides in `docs/`
- UI improvements to the service graph or anomaly feed
- Performance improvements to the collector

## What we're not looking for (in v1)

- Multi-tenancy or auth
- Cloud-specific integrations (AWS X-Ray, GCP Trace, etc.)
- ML-based detection (the statistical approach is intentional)
- Additional databases beyond ClickHouse

## Getting started

```bash
git clone https://github.com/dasuntheekshanadev/open-trace.git
cd open-trace
docker compose up --build
```

The full stack starts at `http://localhost:3000`. The sample app generates traffic automatically so you can see changes immediately.

## Project structure

```
collector/      Go service — OTLP receiver, graph builder, anomaly detector, API
frontend/       React + TypeScript + D3.js dashboard
sample-app/     Demo microservices (Go) used for testing
docs/           Integration guides and architecture notes
```

## Making a change

1. Fork the repo and create a branch: `git checkout -b my-fix`
2. Make your change
3. Test it: `docker compose up --build` and verify the dashboard works
4. Open a pull request with a short description of what changed and why

## Collector (Go)

```bash
cd collector
go build ./...
go test ./...
```

The collector is a single Go binary. Key packages:

| Package | What it does |
|---|---|
| `internal/receiver` | Parses incoming OTLP HTTP/protobuf spans |
| `internal/graph` | Builds the in-memory service dependency graph |
| `internal/detector` | Rolling-window anomaly detection |
| `internal/analyzer` | Root-cause attribute skew analysis |
| `internal/chstore` | ClickHouse read/write |
| `internal/api` | HTTP API handlers |

## Frontend (React + TypeScript)

```bash
cd frontend
npm install
npm run dev     # dev server at http://localhost:5173
npm run build   # production build
```

The service graph uses D3.js force simulation. The main component is `src/components/ServiceGraph.tsx`.

## Sending a test trace

```bash
# Trigger a failure in the sample payment service
curl -X POST http://localhost:8082/admin/fault -d '{"mode":"always-fail"}'

# Watch anomaly detection fire within ~30 seconds at http://localhost:3000

# Reset
curl -X POST http://localhost:8082/admin/fault -d '{"mode":"none"}'
```

## Code style

- Go: standard `gofmt` formatting, no external linter required
- TypeScript: the project uses TypeScript strict mode; `tsc --noEmit` must pass
- Keep comments minimal — only when the *why* is non-obvious

## Questions

Open an issue. We'll respond promptly.
