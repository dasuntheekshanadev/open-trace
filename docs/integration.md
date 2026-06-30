# Integrating OpenTrace into Your App

OpenTrace sits alongside your existing app. You add one small library to your code that reports trace data. OpenTrace collects it, detects anomalies, and shows you what's going wrong — without changing how your app works.

**Time to set up: under 15 minutes.**

---

## What you need

- Docker and Docker Compose installed on a server or your machine
- Your app running (any language)
- Port `4319` reachable from your app

---

## Step 1 — Run OpenTrace

Clone the repo and start everything with one command:

```bash
git clone https://github.com/your-username/opentrace
cd opentrace
docker compose up
```

Once it's up, open **http://localhost:3000** — you'll see an empty dashboard. That's expected; your services aren't connected yet.

> **Running on a remote server?** Replace `localhost` with your server's IP address everywhere in this guide. Make sure port `4319` is open in your firewall.

---

## Step 2 — Add tracing to your app

OpenTelemetry is the open standard for trace data. You install a small library for your language, give it your service name and the OpenTrace address — it handles everything else automatically. No manual changes needed for HTTP calls between services.

Repeat this for every service in your system. Each one gets a **different service name** — that's how OpenTrace tells them apart.

### Node.js

```bash
npm install @opentelemetry/sdk-node \
            @opentelemetry/auto-instrumentations-node \
            @opentelemetry/exporter-trace-otlp-http
```

Create a file called `tracing.js`:

```js
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-http');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');

const sdk = new NodeSDK({
  serviceName: 'your-service-name',   // change this
  traceExporter: new OTLPTraceExporter({
    url: 'http://localhost:4319/v1/traces',
  }),
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

Start your app with tracing enabled:

```bash
node -r ./tracing.js app.js

# or add to package.json:
# "start": "node -r ./tracing.js app.js"
```

---

### Python

```bash
pip install opentelemetry-sdk \
            opentelemetry-exporter-otlp-proto-http \
            opentelemetry-instrumentation-fastapi   # or -flask
```

Add to the top of your main file:

```python
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource

resource = Resource.create({"service.name": "your-service-name"})
provider = TracerProvider(resource=resource)
provider.add_span_processor(
    BatchSpanProcessor(
        OTLPSpanExporter(endpoint="http://localhost:4319/v1/traces")
    )
)
trace.set_tracer_provider(provider)

# FastAPI
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
FastAPIInstrumentor.instrument_app(app)

# Flask
# from opentelemetry.instrumentation.flask import FlaskInstrumentor
# FlaskInstrumentor().instrument_app(app)
```

---

### Go

```bash
go get go.opentelemetry.io/otel \
       go.opentelemetry.io/otel/sdk \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
```

Create a `tracing.go` file and call `initTracer()` at the start of `main()`:

```go
import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func initTracer() {
    exporter, _ := otlptracehttp.New(context.Background(),
        otlptracehttp.WithEndpoint("localhost:4319"),
        otlptracehttp.WithInsecure(),
    )
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("your-service-name"),
        )),
    )
    otel.SetTracerProvider(tp)
}
```

---

### Java

Java uses an agent JAR — no code changes required at all.

```bash
# Download the agent once
curl -L -o opentelemetry-javaagent.jar \
  https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
```

Start your app with the agent:

```bash
java -javaagent:opentelemetry-javaagent.jar \
     -Dotel.service.name=your-service-name \
     -Dotel.exporter.otlp.endpoint=http://localhost:4319 \
     -jar your-app.jar
```

> Supports Spring Boot, Micronaut, Quarkus, Jersey, and most other frameworks automatically.

---

## Step 3 — Verify it's working

Send a few requests through your app — anything that causes one service to call another. Within about **30 seconds**, your services will appear as nodes in the dashboard at `http://localhost:3000`.

Each line connecting two nodes is a live connection between services:
- **Green** — healthy, error rate below 5%
- **Yellow** — slightly elevated errors or latency
- **Red** — something is wrong

Click any line to see the error rate and latency charts, and a ranked list of what attribute values correlate with failures.

---

## Tips

**Wait for the baseline before triggering faults**
Anomaly detection needs 2–3 minutes of normal traffic to build a baseline. If you introduce a fault immediately after starting, the high error rate becomes the "normal" baseline and nothing gets flagged.

**Use consistent service names**
The service name you set (`serviceName`, `service.name`, etc.) is how OpenTrace identifies each service. If you change it between restarts, it will appear as a new node.

**Docker and Kubernetes networking**
If your app runs in Docker or Kubernetes, `localhost:4319` won't work — the collector is on a different host. Use the host machine's IP, or if your app is in the same Docker Compose network as OpenTrace, use `collector:4319`.

**Data survives restarts**
ClickHouse stores 30 days of edge metrics and 7 days of spans. Restarting the collector (`docker compose restart collector`) rehydrates from storage automatically — anomaly detection resumes without a warmup period.

**Production setup**
For production, run OpenTrace on a dedicated server. Set a strong `CLICKHOUSE_PASSWORD` in `docker-compose.yml`. Only expose port `3000` (the dashboard) publicly — keep `4319` (the trace receiver) internal or behind a VPN.
