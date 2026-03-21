// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) OwnPulse Contributors

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	workflowRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ci_workflow_runs_total",
			Help: "Total number of CI workflow runs by workflow and conclusion.",
		},
		[]string{"workflow", "conclusion"},
	)
	workflowDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ci_workflow_duration_seconds",
			Help:    "Duration of CI workflow runs in seconds.",
			Buckets: []float64{30, 60, 120, 300, 600, 1200, 1800},
		},
		[]string{"workflow"},
	)
	jobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ci_job_duration_seconds",
			Help:    "Duration of CI jobs in seconds.",
			Buckets: []float64{30, 60, 120, 300, 600, 1200, 1800},
		},
		[]string{"job", "conclusion"},
	)
	jobQueue = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ci_job_queue_seconds",
			Help:    "Time CI jobs spent queued in seconds.",
			Buckets: []float64{5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"job"},
	)
)

func init() {
	prometheus.MustRegister(workflowRunsTotal, workflowDuration, jobDuration, jobQueue)
}

type WorkflowRunEvent struct {
	Action      string `json:"action"`
	WorkflowRun struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
		CreatedAt  string `json:"created_at"`
		UpdatedAt  string `json:"updated_at"`
	} `json:"workflow_run"`
}

type WorkflowJobEvent struct {
	Action      string `json:"action"`
	WorkflowJob struct {
		Name        string `json:"name"`
		Conclusion  string `json:"conclusion"`
		CreatedAt   string `json:"created_at"`
		StartedAt   string `json:"started_at"`
		CompletedAt string `json:"completed_at"`
	} `json:"workflow_job"`
}

func main() {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("WEBHOOK_SECRET is required")
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /webhook", handleWebhook([]byte(secret)))

	addr := ":8081"
	log.Printf("ci-metrics listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleWebhook(secret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB max
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		sig := r.Header.Get("X-Hub-Signature-256")
		if !validateSignature(body, sig, secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		eventType := r.Header.Get("X-GitHub-Event")
		switch eventType {
		case "workflow_run":
			var ev WorkflowRunEvent
			if err := json.Unmarshal(body, &ev); err != nil {
				http.Error(w, "parse error", http.StatusBadRequest)
				return
			}
			if ev.Action != "completed" {
				w.WriteHeader(http.StatusOK)
				return
			}
			workflowRunsTotal.WithLabelValues(ev.WorkflowRun.Name, ev.WorkflowRun.Conclusion).Inc()
			if dur := parseDuration(ev.WorkflowRun.CreatedAt, ev.WorkflowRun.UpdatedAt); dur > 0 {
				workflowDuration.WithLabelValues(ev.WorkflowRun.Name).Observe(dur)
			}

		case "workflow_job":
			var ev WorkflowJobEvent
			if err := json.Unmarshal(body, &ev); err != nil {
				http.Error(w, "parse error", http.StatusBadRequest)
				return
			}
			if ev.Action != "completed" {
				w.WriteHeader(http.StatusOK)
				return
			}
			if dur := parseDuration(ev.WorkflowJob.StartedAt, ev.WorkflowJob.CompletedAt); dur > 0 {
				jobDuration.WithLabelValues(ev.WorkflowJob.Name, ev.WorkflowJob.Conclusion).Observe(dur)
			}
			if queueTime := parseDuration(ev.WorkflowJob.CreatedAt, ev.WorkflowJob.StartedAt); queueTime > 0 {
				jobQueue.WithLabelValues(ev.WorkflowJob.Name).Observe(queueTime)
			}

		default:
			// Ignore unknown event types
		}

		w.WriteHeader(http.StatusOK)
	}
}

func validateSignature(payload []byte, signature string, secret []byte) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func parseDuration(start, end string) float64 {
	s, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return 0
	}
	e, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return 0
	}
	return e.Sub(s).Seconds()
}
