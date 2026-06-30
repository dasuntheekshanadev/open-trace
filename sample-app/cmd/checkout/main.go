package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"obs-platform/sample-app/tracing"
)

func main() {
	otlpEndpoint := getEnv("OTLP_ENDPOINT", "localhost:4318")
	port := getEnv("PORT", "8081")
	paymentURL := getEnv("PAYMENT_URL", "http://localhost:8082/charge")
	inventoryURL := getEnv("INVENTORY_URL", "http://localhost:8083/check")

	shutdown, err := tracing.Init("checkout-service", otlpEndpoint)
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer shutdown(context.Background())

	tracer := tracing.Tracer("checkout-service")

	// otelhttp.NewTransport automatically injects the traceparent header
	// into outgoing requests, so child spans link up to the parent trace.
	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	mux := http.NewServeMux()
	mux.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "checkout")
		defer span.End()

		// Call payment service
		paymentReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, paymentURL, nil)
		paymentResp, err := client.Do(paymentReq)
		if err != nil {
			http.Error(w, "payment service unreachable", http.StatusBadGateway)
			return
		}
		defer paymentResp.Body.Close()
		paymentBody, _ := io.ReadAll(paymentResp.Body)

		if paymentResp.StatusCode != http.StatusOK {
			w.WriteHeader(http.StatusPaymentRequired)
			w.Write(paymentBody)
			return
		}

		// Call inventory service
		inventoryReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, inventoryURL, nil)
		inventoryResp, err := client.Do(inventoryReq)
		if err != nil {
			http.Error(w, "inventory service unreachable", http.StatusBadGateway)
			return
		}
		defer inventoryResp.Body.Close()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "order placed"}`))
	})

	handler := otelhttp.NewHandler(mux, "checkout-service")

	log.Printf("checkout-service listening on :%s (otlp endpoint: %s)", port, otlpEndpoint)
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
