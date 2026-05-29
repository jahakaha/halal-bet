package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"halal-bet/internal/client/footballdata"
	"halal-bet/internal/client/sofascore"
	"halal-bet/internal/model"
	"halal-bet/internal/repository"
)

type SyncService struct {
	client      *footballdata.Client
	sofascore   *sofascore.Client
	matches     repository.MatchRepository
	predictions repository.PredictionRepository
}

func NewSyncService(
	client *footballdata.Client,
	sofascoreClient *sofascore.Client,
	matches repository.MatchRepository,
	predictions repository.PredictionRepository,
) *SyncService {
	return &SyncService{
		client:      client,
		sofascore:   sofascoreClient,
		matches:     matches,
		predictions: predictions,
	}
}

func (s *SyncService) SyncWC2026(ctx context.Context) (int, error) {
	apiMatches, err := s.client.GetWC2026Matches(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch wc2026 matches: %w", err)
	}
	return s.upsert(ctx, apiMatches)
}

func (s *SyncService) SyncCLFinal(ctx context.Context) (int, error) {
	apiMatches, err := s.client.GetCLFinal(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch cl final: %w", err)
	}
	return s.upsert(ctx, apiMatches)
}

func (s *SyncService) upsert(ctx context.Context, apiMatches []footballdata.Match) (int, error) {
	matches, err := convertMatches(apiMatches)
	if err != nil {
		return 0, err
	}

	if err := s.matches.Upsert(ctx, matches); err != nil {
		return 0, fmt.Errorf("upsert matches: %w", err)
	}

	if err := s.FinalizeFinishedMatches(ctx); err != nil {
		return 0, fmt.Errorf("finalize finished matches: %w", err)
	}

	return len(matches), nil
}

// FinalizeFinishedMatches calculates and stores points for all finished matches
// that still have unscored predictions. Safe to call multiple times.
func (s *SyncService) FinalizeFinishedMatches(ctx context.Context) error {
	pending, err := s.matches.GetFinishedWithPendingScores(ctx)
	if err != nil {
		return err
	}

	for _, m := range pending {
		preds, err := s.predictions.GetByMatch(ctx, m.ID)
		if err != nil {
			return fmt.Errorf("get predictions for match %d: %w", m.ID, err)
		}

		points := make(map[int64]int, len(preds))
		for _, p := range preds {
			if p.Points != nil {
				continue
			}
			pts := model.CalcPoints(&p, &m)
			if pts == nil {
				continue
			}
			points[p.ID] = *pts
		}

		if len(points) == 0 {
			continue
		}

		if err := s.predictions.UpdatePoints(ctx, m.ID, points); err != nil {
			return fmt.Errorf("update points for match %d: %w", m.ID, err)
		}
	}

	return nil
}

// SyncMatchEvents fetches Sofascore events for a given date, links them to our
// FINISHED matches that have risky bets, updates event flags, and recalculates points.
func (s *SyncService) SyncMatchEvents(ctx context.Context, date time.Time) (int, error) {
	pending, err := s.matches.GetFinishedForEventSync(ctx)
	if err != nil {
		return 0, fmt.Errorf("get pending event matches: %w", err)
	}
	if len(pending) == 0 {
		return 0, nil
	}

	events, err := s.sofascore.GetEventsByDate(ctx, date)
	if err != nil {
		return 0, fmt.Errorf("sofascore events: %w", err)
	}

	updated := 0
	for _, m := range pending {
		event := findEvent(events, m.HomeTeam, m.AwayTeam)
		if event == nil {
			continue
		}

		incidents, err := s.sofascore.GetIncidents(ctx, event.ID)
		if err != nil {
			return updated, fmt.Errorf("sofascore incidents for event %d: %w", event.ID, err)
		}

		hadRed, hadPen, hadOwn := sofascore.ParseEvents(incidents)
		if err := s.matches.UpdateEvents(ctx, m.ID, event.ID, hadRed, hadPen, hadOwn); err != nil {
			return updated, fmt.Errorf("update events for match %d: %w", m.ID, err)
		}
		updated++
	}

	// Recalculate points now that event flags are set.
	if updated > 0 {
		if err := s.FinalizeFinishedMatches(ctx); err != nil {
			return updated, fmt.Errorf("re-finalize after events: %w", err)
		}
	}

	return updated, nil
}

// findEvent matches a Sofascore event to our match by normalizing team names.
func findEvent(events []sofascore.Event, homeTeam, awayTeam string) *sofascore.Event {
	for i, e := range events {
		if nameMatch(e.HomeTeam.Name, homeTeam) && nameMatch(e.AwayTeam.Name, awayTeam) {
			return &events[i]
		}
	}
	return nil
}

func nameMatch(a, b string) bool {
	return strings.Contains(normalizeName(a), normalizeName(b)) ||
		strings.Contains(normalizeName(b), normalizeName(a))
}

func normalizeName(s string) string {
	s = strings.ToLower(s)
	for _, suffix := range []string{" fc", " cf", " sc", " ac", " afc", " fk"} {
		s = strings.TrimSuffix(s, suffix)
	}
	return strings.TrimSpace(s)
}

func convertMatches(apiMatches []footballdata.Match) ([]model.Match, error) {
	matches := make([]model.Match, 0, len(apiMatches))
	for _, am := range apiMatches {
		matchDate, err := time.Parse(time.RFC3339, am.UTCDate)
		if err != nil {
			return nil, fmt.Errorf("parse match date %q: %w", am.UTCDate, err)
		}

		matches = append(matches, model.Match{
			ExternalID: am.ID,
			HomeTeam:   am.HomeTeam.Name,
			AwayTeam:   am.AwayTeam.Name,
			MatchDate:  matchDate.UTC(),
			Status:     model.MatchStatus(am.Status),
			HomeScore:  am.Score.FullTime.Home,
			AwayScore:  am.Score.FullTime.Away,
			Stage:      am.Stage,
			Group:      am.Group,
			Matchday:   am.Matchday,
		})
	}
	return matches, nil
}
