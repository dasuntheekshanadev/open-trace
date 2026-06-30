package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"obs-platform/collector/internal/api"
	"obs-platform/collector/internal/chstore"
	"obs-platform/collector/internal/detector"
	"obs-platform/collector/internal/graph"
	"obs-platform/collector/internal/receiver"
	"obs-platform/collector/internal/store"
)

func main() {
	port := getEnv("PORT", "4319")
	jaegerEndpoint := getEnv("JAEGER_ENDPOINT", "")
	clickhouseURL := getEnv("CLICKHOUSE_URL", "")

	g := graph.New()
	st := store.New()

	var ch *chstore.Client
	if clickhouseURL != "" {
		ch = chstore.New(clickhouseURL)
		log.Printf("waiting for ClickHouse at %s …", clickhouseURL)
		if err := ch.WaitReady(60 * time.Second); err != nil {
			log.Fatalf("clickhouse not ready: %v", err)
		}
		if err := ch.Init(); err != nil {
			log.Fatalf("clickhouse init: %v", err)
		}
		log.Printf("ClickHouse ready — rehydrating state …")
		st.Rehydrate(ch)
	}

	d := detector.New(g, ch)
	if ch != nil {
		d.Rehydrate()
	}
	d.Start()

	a := api.New(g, d, st, ch)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", receiver.New(g, st, ch, jaegerEndpoint).Handle)
	mux.HandleFunc("/graph", a.HandleGraph)
	mux.HandleFunc("/anomalies", a.HandleAnomalies)
	mux.HandleFunc("/rootcause", a.HandleRootCause)
	mux.HandleFunc("/timeseries", a.HandleTimeSeries)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("collector listening on :%s (jaeger: %s, clickhouse: %s)", port, jaegerEndpoint, clickhouseURL)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
