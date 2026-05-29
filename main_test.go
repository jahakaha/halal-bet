package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// buildRouter mirrors the HTTP routing in main() without DB/bot dependencies.
func buildRouter(syncFn func(w http.ResponseWriter, r *http.Request), eventsFn func(w http.ResponseWriter, r *http.Request)) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Post("/sync/wc2026", syncFn)
	router.Post("/sync/cl-final", syncFn)
	router.Post("/sync/events", eventsFn)
	return router
}

func TestHealthEndpoint(t *testing.T) {
	r := buildRouter(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSyncEndpoint_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "synced 5 matches\n")
	}
	r := buildRouter(handler, nil)

	for _, path := range []string{"/sync/wc2026", "/sync/cl-final"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, w.Code)
		}
		if !strings.Contains(w.Body.String(), "synced") {
			t.Errorf("%s: unexpected body: %q", path, w.Body.String())
		}
	}
}

func TestSyncEndpoint_Error(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "api down", http.StatusInternalServerError)
	}
	r := buildRouter(handler, nil)

	req := httptest.NewRequest(http.MethodPost, "/sync/wc2026", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestEventsEndpoint_InvalidDate(t *testing.T) {
	eventsHandler := func(w http.ResponseWriter, r *http.Request) {
		dateStr := r.URL.Query().Get("date")
		if dateStr == "" {
			fmt.Fprintf(w, "synced events for 0 matches\n")
			return
		}
		if _, err := time.Parse("2006-01-02", dateStr); err != nil {
			http.Error(w, "invalid date, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "synced events for 1 matches\n")
	}
	r := buildRouter(nil, eventsHandler)

	t.Run("no date defaults to today", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/sync/events", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("valid date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/sync/events?date=2026-06-15", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("invalid date format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/sync/events?date=15-06-2026", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestMethodNotAllowed(t *testing.T) {
	r := buildRouter(
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
	)

	// GET on a POST-only endpoint should return 405
	req := httptest.NewRequest(http.MethodGet, "/sync/wc2026", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
