// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) OwnPulse Contributors

package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type request struct {
	Name  string `json:"name"`
	Email string `json:"email"`
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
	mux.HandleFunc("POST /signup", handleSignup(db))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	addr := ":8080"
	log.Printf("waitlist listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS waitlist (
			id         BIGSERIAL PRIMARY KEY,
			email      TEXT NOT NULL UNIQUE,
			name       TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
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

		_, err := db.Exec(
			`INSERT INTO waitlist (email, name) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING`,
			req.Email, req.Name,
		)
		if err != nil {
			log.Printf("insert waitlist: %v", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

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
