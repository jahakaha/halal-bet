package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/client/footballdata"
	"halal-bet/internal/client/sofascore"
	"halal-bet/internal/model"
	"halal-bet/internal/repository"
)

type sofascoreClient interface {
	GetWC2026Events(ctx context.Context, page int) ([]sofascore.Event, error)
	GetIncidents(ctx context.Context, eventID int64) ([]sofascore.Incident, error)
}

type SyncService struct {
	client      *footballdata.Client
	sofascore   sofascoreClient
	matches     repository.MatchRepository
	predictions repository.PredictionRepository

	bot     *tele.Bot
	adminID int64

	eventCacheMu  sync.Mutex
	eventCache    []sofascore.Event
	eventCachedAt time.Time
}

func NewSyncService(
	client *footballdata.Client,
	sofascoreClient *sofascore.Client,
	matches repository.MatchRepository,
	predictions repository.PredictionRepository,
	bot *tele.Bot,
	adminID int64,
) *SyncService {
	return &SyncService{
		client:      client,
		sofascore:   sofascoreClient,
		matches:     matches,
		predictions: predictions,
		bot:         bot,
		adminID:     adminID,
	}
}

// alert sends a Telegram DM to the admin and also logs the message.
func (s *SyncService) alert(msg string) {
	log.Printf("ALERT: %s", msg)
	if s.bot == nil || s.adminID == 0 {
		return
	}
	s.bot.Send(&tele.Chat{ID: s.adminID}, "⚠️ HalalBet alert:\n"+msg) //nolint:errcheck
}

// cachedWC2026Events returns the Sofascore event list, refreshing at most once per 12 hours.
// This caps GetWC2026Events API calls to ~2/day regardless of how often SyncMatchEvents runs.
func (s *SyncService) cachedWC2026Events(ctx context.Context) ([]sofascore.Event, error) {
	s.eventCacheMu.Lock()
	defer s.eventCacheMu.Unlock()

	if time.Since(s.eventCachedAt) < 12*time.Hour && len(s.eventCache) > 0 {
		return s.eventCache, nil
	}

	var all []sofascore.Event
	for page := 0; page <= 10; page++ {
		events, err := s.sofascore.GetWC2026Events(ctx, page)
		if err != nil {
			s.alert(fmt.Sprintf("sofascore GetWC2026Events page %d: %v", page, err))
			return nil, fmt.Errorf("sofascore wc2026 events page %d: %w", page, err)
		}
		all = append(all, events...)
		if len(events) == 0 {
			break
		}
	}

	s.eventCache = all
	s.eventCachedAt = time.Now()
	log.Printf("sync: refreshed sofascore event cache: %d events", len(all))
	return all, nil
}

func (s *SyncService) SyncWC2026(ctx context.Context) (int, error) {
	apiMatches, err := s.client.GetWC2026Matches(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch wc2026 matches: %w", err)
	}
	n, err := s.upsert(ctx, apiMatches)
	if err != nil {
		return n, err
	}

	for _, status := range []string{"IN_PLAY", "FINISHED", "PAUSED"} {
		live, err := s.client.GetWC2026ByStatus(ctx, status)
		if err != nil {
			log.Printf("sync: fetch wc2026 %s: %v", status, err)
			continue
		}
		if len(live) > 0 {
			if _, err := s.upsert(ctx, live); err != nil {
				log.Printf("sync: upsert wc2026 %s: %v", status, err)
			}
		}
	}

	return n, nil
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

// SyncMatchEvents fetches WC2026 events from Sofascore, links them to our
// FINISHED matches that have risky bets, updates event flags, and recalculates points.
func (s *SyncService) SyncMatchEvents(ctx context.Context) (int, error) {
	pending, err := s.matches.GetFinishedForEventSync(ctx)
	if err != nil {
		return 0, fmt.Errorf("get pending event matches: %w", err)
	}
	if len(pending) == 0 {
		return 0, nil
	}

	allEvents, err := s.cachedWC2026Events(ctx)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, m := range pending {
		event := findEvent(allEvents, m.HomeTeam, m.AwayTeam)
		if event == nil {
			continue
		}

		incidents, err := s.sofascore.GetIncidents(ctx, event.ID)
		if err != nil {
			s.alert(fmt.Sprintf("GetIncidents для матча %s–%s (event %d): %v", m.HomeTeam, m.AwayTeam, event.ID, err))
			return updated, fmt.Errorf("sofascore incidents for event %d: %w", event.ID, err)
		}

		hadRed, hadPen, hadOwn := sofascore.ParseEvents(incidents)
		if err := s.matches.UpdateEvents(ctx, m.ID, event.ID, hadRed, hadPen, hadOwn); err != nil {
			return updated, fmt.Errorf("update events for match %d: %w", m.ID, err)
		}
		if err := s.predictions.ResetPoints(ctx, m.ID); err != nil {
			return updated, fmt.Errorf("reset points for match %d: %w", m.ID, err)
		}
		updated++
	}

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
	na, nb := normalizeName(a), normalizeName(b)
	if na == nb {
		return true
	}
	// only allow substring match when the shorter name is at least 5 chars
	// to avoid false positives like "iran" inside "ukraine"
	shorter, longer := na, nb
	if len(na) > len(nb) {
		shorter, longer = nb, na
	}
	return len(shorter) >= 5 && strings.Contains(longer, shorter)
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
