package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	gatewayURL := getEnv("GATEWAY_URL", "http://localhost:8080/checkout")
	intervalMs := 200 // one request every 200ms by default

	log.Printf("load generator hitting %s every %dms", gatewayURL, intervalMs)

	for {
		resp, err := http.Post(gatewayURL, "application/json", nil)
		if err != nil {
			log.Printf("request failed: %v", err)
		} else {
			log.Printf("checkout -> %d", resp.StatusCode)
			resp.Body.Close()
		}
		time.Sleep(time.Duration(intervalMs) * time.Millisecond)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
