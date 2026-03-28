// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) OwnPulse Contributors

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	signupsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "waitlist_signups_total",
		Help: "Total number of waitlist signup attempts.",
	}, []string{"status"})

	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "waitlist_http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status_code"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "waitlist_http_request_duration_seconds",
		Help: "HTTP request duration in seconds.",
	}, []string{"method", "path"})
)

func init() {
	prometheus.MustRegister(signupsTotal, httpRequestsTotal, httpRequestDuration)
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

var validPersonas = map[string]bool{
	"quantified_self": true, "biohacker": true, "peptide_pioneer": true,
	"iron_scientist": true, "health_detective": true, "builder": true,
	"clinician": true, "basics": true,
}

type request struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Persona string `json:"persona"`
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/waitlist", handleSignup(db))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	addr := ":8080"
	log.Printf("waitlist listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, metricsMiddleware(mux)))
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS waitlist (
			id         BIGSERIAL PRIMARY KEY,
			email      TEXT NOT NULL UNIQUE,
			name       TEXT NOT NULL DEFAULT '',
			persona    TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE waitlist ADD COLUMN IF NOT EXISTS persona TEXT`)
	if err != nil {
		return err
	}

	roPass := os.Getenv("GRAFANA_RO_PASSWORD")
	if roPass != "" {
		escapedPass := strings.ReplaceAll(roPass, "'", "''")
		stmts := []string{
			`DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'grafana_ro') THEN CREATE ROLE grafana_ro; END IF; END $$`,
			fmt.Sprintf(`ALTER ROLE grafana_ro WITH LOGIN PASSWORD '%s'`, escapedPass),
			`GRANT CONNECT ON DATABASE waitlist TO grafana_ro`,
			`GRANT SELECT ON ALL TABLES IN SCHEMA public TO grafana_ro`,
			`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO grafana_ro`,
		}
		for _, s := range stmts {
			if _, err := db.Exec(s); err != nil {
				return fmt.Errorf("grafana_ro setup: %w", err)
			}
		}
	}

	return nil
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", sw.code)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

func handleSignup(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
			jsonError(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.Name = strings.TrimSpace(req.Name)

		if !emailRe.MatchString(req.Email) {
			jsonError(w, "invalid email", http.StatusBadRequest)
			return
		}

		var persona *string
		if req.Persona != "" {
			if !validPersonas[req.Persona] {
				jsonError(w, "invalid persona", http.StatusBadRequest)
				return
			}
			persona = &req.Persona
		}

		_, err := db.Exec(
			`INSERT INTO waitlist (email, name, persona) VALUES ($1, $2, $3) ON CONFLICT (email) DO UPDATE SET persona = COALESCE(EXCLUDED.persona, waitlist.persona)`,
			req.Email, req.Name, persona,
		)
		if err != nil {
			log.Printf("insert waitlist: %v", err)
			signupsTotal.WithLabelValues("error").Inc()
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

		signupsTotal.WithLabelValues("created").Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
