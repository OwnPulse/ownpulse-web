// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) OwnPulse Contributors

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		// Skip all tests gracefully when no database is available.
		os.Exit(0)
	}

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		panic("open db: " + err.Error())
	}
	if err := testDB.Ping(); err != nil {
		panic("ping db: " + err.Error())
	}
	if err := migrate(testDB); err != nil {
		panic("migrate: " + err.Error())
	}

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

// newTestServer builds the same mux as main() and wraps it in httptest.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/waitlist", handleSignup(testDB))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := testDB.Ping(); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// cleanupEmail removes a waitlist row by email so tests stay independent.
func cleanupEmail(t *testing.T, email string) {
	t.Helper()
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM waitlist WHERE email = $1", email)
	})
}

func postJSON(url string, body any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	return http.Post(url+"/api/waitlist", "application/json", bytes.NewReader(b))
}

func TestSignupSuccess(t *testing.T) {
	ts := newTestServer(t)
	email := "success@example.com"
	cleanupEmail(t, email)

	resp, err := postJSON(ts.URL, map[string]string{"email": email})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]bool
	json.NewDecoder(resp.Body).Decode(&result)
	if !result["ok"] {
		t.Fatal("expected ok: true")
	}
}

func TestSignupWithPersona(t *testing.T) {
	ts := newTestServer(t)
	email := "persona@example.com"
	cleanupEmail(t, email)

	resp, err := postJSON(ts.URL, map[string]string{
		"email":   email,
		"persona": "biohacker",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestSignupInvalidEmail(t *testing.T) {
	ts := newTestServer(t)

	resp, err := postJSON(ts.URL, map[string]string{"email": "not-an-email"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "invalid email" {
		t.Fatalf("expected 'invalid email' error, got %q", result["error"])
	}
}

func TestSignupInvalidPersona(t *testing.T) {
	ts := newTestServer(t)

	resp, err := postJSON(ts.URL, map[string]string{
		"email":   "badpersona@example.com",
		"persona": "nonexistent",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "invalid persona" {
		t.Fatalf("expected 'invalid persona' error, got %q", result["error"])
	}
}

func TestSignupDuplicate(t *testing.T) {
	ts := newTestServer(t)
	email := "duplicate@example.com"
	cleanupEmail(t, email)

	// First signup.
	resp1, err := postJSON(ts.URL, map[string]string{"email": email})
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("first: expected 201, got %d", resp1.StatusCode)
	}

	// Second signup with same email — should be idempotent.
	resp2, err := postJSON(ts.URL, map[string]string{"email": email})
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("second: expected 201, got %d", resp2.StatusCode)
	}
}

func TestSignupMissingEmail(t *testing.T) {
	ts := newTestServer(t)

	cases := []struct {
		name string
		body any
	}{
		{"empty object", map[string]string{}},
		{"missing email field", map[string]string{"name": "Alice"}},
		{"empty email", map[string]string{"email": ""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := postJSON(ts.URL, tc.body)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
