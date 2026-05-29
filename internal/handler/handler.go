package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type SyncService interface {
	SyncWC2026(ctx context.Context) (int, error)
	SyncCLFinal(ctx context.Context) (int, error)
	SyncMatchEvents(ctx context.Context, date time.Time) (int, error)
}

type Handler struct {
	sync SyncService
}

func New(sync SyncService) *Handler {
	return &Handler{sync: sync}
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/health", h.health)
	r.Post("/sync/wc2026", h.syncWC2026)
	r.Post("/sync/cl-final", h.syncCLFinal)
	r.Post("/sync/events", h.syncEvents)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) syncWC2026(w http.ResponseWriter, r *http.Request) {
	n, err := h.sync.SyncWC2026(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "synced %d matches\n", n)
}

func (h *Handler) syncCLFinal(w http.ResponseWriter, r *http.Request) {
	n, err := h.sync.SyncCLFinal(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "synced %d matches\n", n)
}

func (h *Handler) syncEvents(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	var err error
	if dateStr == "" {
		date = time.Now().UTC()
	} else {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}
	n, err := h.sync.SyncMatchEvents(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "synced events for %d matches\n", n)
}
