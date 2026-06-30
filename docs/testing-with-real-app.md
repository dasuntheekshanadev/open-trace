# Testing OpenTrace with a Real App

This guide connects OpenTrace to the **OpenTelemetry Demo** — Google's official
microservices demo app with 10+ real services written in different languages, all
already instrumented with OpenTelemetry. You don't need to change any application
code at all.

---

## What you need

- OpenTrace already running (`docker compose up` in the `open-trace` folder)
- Docker and Docker Compose
- ~4 GB of free RAM (the demo has many services)

---

## Step 1 — Find your host IP

OpenTrace runs in one Docker Compose stack. The OTel demo runs in another. They
need a way to talk to each other. The simplest way is to use your machine's IP
address so Docker containers in one stack can reach the other stack.

```bash
hostname -I | awk '{print $1}'
```

Write that IP down — you'll use it in Step 3. It will look like `192.168.x.x`
or `10.x.x.x`.

> **Tip:** Don't use `localhost` or `127.0.0.1` here — those refer to inside the
> container, not your machine. You need the real network IP.

---

## Step 2 — Clone the OpenTelemetry Demo

```bash
git clone https://github.com/open-telemetry/opentelemetry-demo.git
cd opentelemetry-demo
```

This demo includes services written in Go, Python, Java, Node.js, .NET, Ruby,
and more — all talking to each other over HTTP and gRPC.

---

## Step 3 — Point the demo's traces at OpenTrace

The demo runs its own internal OTel Collector that receives traces from all
services. We'll add a forwarding rule so that collector also sends everything
to OpenTrace.

The collector config file is at:

```
src/otelcollector/otelcol-config.yml
```

Open it and find the `exporters:` section. Add an `otlphttp` exporter pointing
at your OpenTrace collector (replace `YOUR_HOST_IP` with the IP from Step 1):

```yaml
exporters:
  # ... existing exporters stay here, just ADD these lines below them:
  otlphttp/opentrace:
    endpoint: "http://YOUR_HOST_IP:4319"
    tls:
      insecure: true
```

Then find the `pipelines:` section inside `service:`. Find the `traces:` pipeline
and add `otlphttp/opentrace` to its `exporters` list:

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp, otlphttp/opentrace]   # <-- add otlphttp/opentrace here
```

Save the file.

---

## Step 4 — Start the demo

```bash
docker compose up
```

The demo takes 2–3 minutes to fully start because it's building and booting
10+ services. You'll know it's ready when the logs stop printing errors and
settle down.

The demo's own frontend (a fake online shop) is at **http://localhost:8080**.
Open it and click around — this generates realistic traffic between services.

---

## Step 5 — Open OpenTrace

Go to **http://localhost:3000**

Within 30 seconds you'll see the demo's services appear as nodes in the graph.
The demo generates continuous traffic on its own (via a load generator), so
edges and stats will populate quickly.

You should see services like:
- `frontend` → `cartservice`
- `frontend` → `productcatalogservice`
- `checkoutservice` → `paymentservice`
- `checkoutservice` → `emailservice`
- and more...

---

## Step 6 — Trigger an anomaly (optional)

The demo has a feature flag system you can use to inject failures.

```bash
# Open the feature flag UI
open http://localhost:8081
```

Enable the `productCatalogFailure` flag — this makes the product catalog service
start returning errors on some requests. Watch OpenTrace detect it within
30–60 seconds and flag the `frontend → productcatalogservice` edge.

Click that edge to see:
- The error rate spike in the time-series chart
- The root-cause analysis table showing which attribute value correlates with failures

---

## Troubleshooting

**Services don't appear in OpenTrace after 2 minutes**

The most common cause is the IP address. Double-check that the IP you used
in `otelcol-config.yml` is reachable from inside a Docker container:

```bash
# From your machine, run a quick test container
docker run --rm curlimages/curl curl -s http://YOUR_HOST_IP:4319/healthz
```

If it returns `200 OK`, the address is correct. If it hangs or errors,
try a different IP from `hostname -I` output.

**"connection refused" errors in the demo's collector logs**

Make sure OpenTrace is fully started before the demo tries to connect.
Check with:

```bash
curl http://localhost:4319/healthz
```

If it returns `OK`, OpenTrace is ready.

**Demo services show as one big blob with no edges**

The demo uses gRPC between some services, which uses binary framing that
OpenTrace doesn't currently decode for edge resolution. HTTP-based edges
will still appear correctly. You'll see the clearest graph from services
that communicate over HTTP.

---

## After testing

Stop the demo when you're done:

```bash
# In the opentelemetry-demo directory
docker compose down
```

OpenTrace keeps running and retains all the data in ClickHouse. You can
restart the demo later and the baseline will already be established.

---

## Testing with your own app

If you have your own app (Node.js, Python, Go, or Java), see
**[integration.md](integration.md)** for how to add OpenTelemetry to it in
under 15 minutes. Once traces are flowing, everything in this guide applies —
the dashboard, anomaly detection, and root-cause analysis all work the same way.
