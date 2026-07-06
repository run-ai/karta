// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation
//
// Fictive CPU stand-in for an NVIDIA NIM container, used by the Karta e2e suite.
// It answers the HTTP endpoints the k8s-nim-operator probes (in particular
// GET /v1/health/ready) so the operator drives the NIMService to state=Ready.
// It loads no model and performs no inference.
//
// Standard library only: the image is a static binary on a distroless base, so
// it carries no Python runtime or third-party package CVEs.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const modelName = "meta/llama-3.2-1b-instruct"

// requestLatency is applied to the chat endpoint to imitate processing time
// (NIM_REQUEST_LATENCY, seconds). Readiness is always immediate.
var requestLatency = latencyFromEnv()

func main() {
	port := getenv("NIM_HTTP_PORT", "8000")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health/ready", status("ready"))
	mux.HandleFunc("/v1/health/live", status("alive"))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"status": "healthy"})
	})
	mux.HandleFunc("/v1/models", listModels)
	mux.HandleFunc("/v1/chat/completions", chatCompletions)
	mux.HandleFunc("/v1/metrics", metrics)
	mux.HandleFunc("/", root)

	addr := ":" + port
	log.Printf("fictive NIM listening on %s (model %s, latency %s)", addr, modelName, requestLatency)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func status(state string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{
			"status":    state,
			"model":     modelName,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func listModels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"object": "list",
		"data": []any{map[string]any{
			"id":       modelName,
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": "nim-service",
		}},
	})
}

func chatCompletions(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(requestLatency)
	writeJSON(w, map[string]any{
		"id":      "chatcmpl-fictive",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   modelName,
		"choices": []any{map[string]any{
			"index":         0,
			"finish_reason": "stop",
			"message":       map[string]string{"role": "assistant", "content": "This is a simulated NIM response."},
		}},
		"usage": map[string]int{"prompt_tokens": 50, "completion_tokens": 25, "total_tokens": 75},
	})
}

func metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte("# HELP num_requests_running Number of requests currently running\n" +
		"# TYPE num_requests_running gauge\n" +
		"num_requests_running 0\n" +
		"# HELP gpu_cache_usage_perc GPU cache usage percentage\n" +
		"# TYPE gpu_cache_usage_perc gauge\n" +
		"gpu_cache_usage_perc 0\n"))
}

func root(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"message":   "fictive NVIDIA NIM is running",
		"model":     modelName,
		"endpoints": []string{"/v1/models", "/v1/chat/completions", "/v1/health/ready", "/v1/health/live", "/v1/metrics"},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func latencyFromEnv() time.Duration {
	if v := os.Getenv("NIM_REQUEST_LATENCY"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return time.Duration(f * float64(time.Second))
		}
	}
	return 0
}
