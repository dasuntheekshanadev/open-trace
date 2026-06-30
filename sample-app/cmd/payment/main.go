package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"obs-platform/sample-app/tracing"
)

// faultMode is toggled via /admin/fault to let you simulate incidents on demand
// while you're learning/testing the anomaly detector and root-cause engine.
//   "none"        - normal baseline behavior (~2% errors, low latency)
//   "pod-2-down"  - one simulated "pod" fails almost every request (root-cause demo)
//   "global-slow" - everything gets slow (latency anomaly demo)
//   "global-fail" - elevated error rate across the board
var faultMode atomic.Value

func main() {
	faultMode.Store("none")

	otlpEndpoint := getEnv("OTLP_ENDPOINT", "localhost:4318")
	port := getEnv("PORT", "8082")

	shutdown, err := tracing.Init("payment-service", otlpEndpoint)
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer shutdown(context.Background())

	tracer := tracing.Tracer("payment-service")

	mux := http.NewServeMux()

	mux.HandleFunc("/charge", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "process-payment")
		defer span.End()

		// Simulate the request landing on one of 3 "pods" behind a load balancer.
		// This attribute is what the future root-cause engine will learn to spot.
		pod := "payment-" + []string{"1", "2", "3"}[rand.Intn(3)]
		span.SetAttributes(attribute.String("payment.pod", pod))

		mode := faultMode.Load().(string)

		baseLatency := 30 + rand.Intn(60) // 30-90ms baseline
		errorChance := 0.02               // 2% baseline error rate

		switch mode {
		case "pod-2-down":
			if pod == "payment-2" {
				errorChance = 0.9 // pod-2 is effectively down
			}
		case "global-slow":
			baseLatency += 400
		case "global-fail":
			errorChance = 0.4
		}

		time.Sleep(time.Duration(baseLatency) * time.Millisecond)

		span.SetAttributes(
			attribute.Int("payment.latency_ms", baseLatency),
			attribute.String("payment.fault_mode", mode),
		)

		if rand.Float64() < errorChance {
			span.SetStatus(codes.Error, "payment declined: downstream bank timeout")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"status": "failed", "pod": "` + pod + `"}`))
			return
		}

		_ = ctx
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "approved", "pod": "` + pod + `"}`))
	})

	// Lets you flip incident modes on demand while testing the detector,
	// e.g.: curl -X POST localhost:8082/admin/fault -d "pod-2-down"
	mux.HandleFunc("/admin/fault", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			buf := make([]byte, 64)
			n, _ := r.Body.Read(buf)
			mode := string(buf[:n])
			if mode == "" {
				mode = "none"
			}
			faultMode.Store(mode)
			log.Printf("fault mode set to: %s", mode)
		}
		w.Write([]byte(faultMode.Load().(string)))
	})

	handler := otelhttp.NewHandler(mux, "payment-service")

	log.Printf("payment-service listening on :%s (otlp endpoint: %s)", port, otlpEndpoint)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
