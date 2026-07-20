package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"halal-bet/internal/model"
)

type SyncService interface {
	SyncWC2026(ctx context.Context) (int, error)
	SyncMatchEvents(ctx context.Context) (int, error)
}

type TournamentRepository interface {
	GetSummary(ctx context.Context, groupID int64) ([]model.TournamentPredictionSummary, error)
}

type Handler struct {
	sync       SyncService
	tournament TournamentRepository
}

func New(sync SyncService, tournament TournamentRepository) *Handler {
	return &Handler{sync: sync, tournament: tournament}
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/health", h.health)
	r.Post("/sync/wc2026", h.syncWC2026)
	r.Post("/sync/events", h.syncEvents)
	r.Get("/groups/{groupID}/tournament/predictions", h.tournamentPredictions)
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

func (h *Handler) syncEvents(w http.ResponseWriter, r *http.Request) {
	n, err := h.sync.SyncMatchEvents(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "synced events for %d matches\n", n)
}

// tournamentPredictions returns each user's tournament choices and points.
func (h *Handler) tournamentPredictions(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil || groupID <= 0 {
		http.Error(w, "invalid groupID", http.StatusBadRequest)
		return
	}

	predictions, err := h.tournament.GetSummary(r.Context(), groupID)
	if err != nil {
		http.Error(w, "get tournament predictions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(predictions); err != nil {
		http.Error(w, "encode tournament predictions: "+err.Error(), http.StatusInternalServerError)
	}
}
