package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"

	"obs-platform/sample-app/tracing"
)

func main() {
	otlpEndpoint := getEnv("OTLP_ENDPOINT", "localhost:4318")
	port := getEnv("PORT", "8083")

	shutdown, err := tracing.Init("inventory-service", otlpEndpoint)
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer shutdown(context.Background())

	tracer := tracing.Tracer("inventory-service")

	mux := http.NewServeMux()
	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "check-inventory")
		defer span.End()

		// Inventory is the most reliable service in the chain — mostly fast,
		// occasionally a touch slow under "load", almost never errors.
		latency := 10 + rand.Intn(30) // 10-40ms baseline
		if rand.Float64() < 0.03 {    // 3% chance of a slow check (DB contention)
			latency += 150
			span.SetAttributes(attribute.Bool("inventory.slow_path", true))
		}
		time.Sleep(time.Duration(latency) * time.Millisecond)

		span.SetAttributes(
			attribute.String("inventory.item", "sku-12345"),
			attribute.Int("inventory.latency_ms", latency),
		)

		_ = ctx
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"available": true}`))
	})

	handler := otelhttp.NewHandler(mux, "inventory-service")

	log.Printf("inventory-service listening on :%s (otlp endpoint: %s)", port, otlpEndpoint)
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
