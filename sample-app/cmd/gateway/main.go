package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"obs-platform/sample-app/tracing"
)

func main() {
	otlpEndpoint := getEnv("OTLP_ENDPOINT", "localhost:4318")
	port := getEnv("PORT", "8080")
	checkoutURL := getEnv("CHECKOUT_URL", "http://localhost:8081/checkout")

	shutdown, err := tracing.Init("gateway", otlpEndpoint)
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer shutdown(context.Background())

	tracer := tracing.Tracer("gateway")
	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	mux := http.NewServeMux()
	mux.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "gateway-handle-checkout")
		defer span.End()

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, checkoutURL, nil)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "checkout service unreachable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := otelhttp.NewHandler(mux, "gateway")

	log.Printf("gateway listening on :%s (otlp endpoint: %s)", port, otlpEndpoint)
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
