package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/client/apifootball"
	"halal-bet/internal/client/footballdata"
	"halal-bet/internal/model"
	"halal-bet/internal/repository"
)

type apiFootballClient interface {
	GetFixturesByDate(ctx context.Context, date time.Time) ([]apifootball.Fixture, error)
	GetFixtureEvents(ctx context.Context, fixtureID int64) ([]apifootball.Event, error)
}

type SyncService struct {
	client      *footballdata.Client
	apifootball apiFootballClient
	matches     repository.MatchRepository
	predictions repository.PredictionRepository

	bot     *tele.Bot
	adminID int64

	// fixtureByDate caches API-Football fixtures per calendar date (UTC).
	// Dates don't change so entries are never evicted.
	fixtureCacheMu sync.Mutex
	fixtureByDate  map[string][]apifootball.Fixture
}

func NewSyncService(
	client *footballdata.Client,
	apiFootball *apifootball.Client,
	matches repository.MatchRepository,
	predictions repository.PredictionRepository,
	bot *tele.Bot,
	adminID int64,
) *SyncService {
	return &SyncService{
		client:        client,
		apifootball:   apiFootball,
		matches:       matches,
		predictions:   predictions,
		bot:           bot,
		adminID:       adminID,
		fixtureByDate: make(map[string][]apifootball.Fixture),
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

// wc2026Start is the first match day of WC 2026. Dates before this are skipped.
var wc2026Start = time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)

// fixturesForDate returns World Cup fixtures for the given UTC date, cached forever
// (match dates don't change). Uses GET /fixtures?date=YYYY-MM-DD which works on the free plan.
// Returns nil (no error) for dates before WC2026 start — free plan returns 403 for old dates.
func (s *SyncService) fixturesForDate(ctx context.Context, date time.Time) ([]apifootball.Fixture, error) {
	if date.UTC().Before(wc2026Start) {
		log.Printf("sync: skipping fixture lookup for pre-WC2026 date %s", date.UTC().Format("2006-01-02"))
		return nil, nil
	}

	key := date.UTC().Format("2006-01-02")

	s.fixtureCacheMu.Lock()
	defer s.fixtureCacheMu.Unlock()

	if cached, ok := s.fixtureByDate[key]; ok {
		return cached, nil
	}

	fixtures, err := s.apifootball.GetFixturesByDate(ctx, date)
	if err != nil {
		// 403 = free plan restriction on old dates — log only, no alert
		log.Printf("sync: apifootball GetFixturesByDate %s: %v", key, err)
		return nil, nil
	}

	s.fixtureByDate[key] = fixtures
	log.Printf("sync: cached %d apifootball fixtures for %s", len(fixtures), key)
	return fixtures, nil
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

// SyncMatchEvents fetches WC2026 fixture events from API-Football, links them to our
// FINISHED matches that have risky bets, updates event flags, and recalculates points.
func (s *SyncService) SyncMatchEvents(ctx context.Context) (int, error) {
	pending, err := s.matches.GetFinishedForEventSync(ctx)
	if err != nil {
		return 0, fmt.Errorf("get pending event matches: %w", err)
	}
	if len(pending) == 0 {
		return 0, nil
	}

	updated := 0
	for _, m := range pending {
		fixtures, err := s.fixturesForDate(ctx, m.MatchDate)
		if err != nil {
			return updated, err
		}

		fixture := findFixture(fixtures, m.HomeTeam, m.AwayTeam)
		if fixture == nil {
			log.Printf("sync: no apifootball fixture found for %s–%s on %s", m.HomeTeam, m.AwayTeam, m.MatchDate.Format("2006-01-02"))
			continue
		}

		events, err := s.apifootball.GetFixtureEvents(ctx, fixture.ID)
		if err != nil {
			s.alert(fmt.Sprintf("GetFixtureEvents для матча %s–%s (fixture %d): %v", m.HomeTeam, m.AwayTeam, fixture.ID, err))
			return updated, fmt.Errorf("apifootball events for fixture %d: %w", fixture.ID, err)
		}

		hadRed, hadPen, hadOwn := apifootball.ParseEvents(events)
		if err := s.matches.UpdateEvents(ctx, m.ID, fixture.ID, hadRed, hadPen, hadOwn); err != nil {
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

// findFixture matches an API-Football fixture to our match by normalizing team names.
func findFixture(fixtures []apifootball.Fixture, homeTeam, awayTeam string) *apifootball.Fixture {
	for i, f := range fixtures {
		if nameMatch(f.HomeTeam, homeTeam) && nameMatch(f.AwayTeam, awayTeam) {
			return &fixtures[i]
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

var nameAliases = map[string]string{
	"türkiye":              "turkey",
	"usa":                  "united states",
	"bosnia & herzegovina": "bosnia-herzegovina",
	"ir iran":              "iran",
	"korea republic":       "south korea",
	"côte d'ivoire":        "ivory coast",
}

func normalizeName(s string) string {
	s = strings.ToLower(s)
	for _, suffix := range []string{" fc", " cf", " sc", " ac", " afc", " fk"} {
		s = strings.TrimSuffix(s, suffix)
	}
	s = strings.TrimSpace(s)
	if alias, ok := nameAliases[s]; ok {
		return alias
	}
	return s
}


// regulationScore returns the 90-minute score for betting purposes.
//
// The football-data.org v4 API:
//   - REGULAR matches: fullTime = 90-min score (correct as-is)
//   - EXTRA_TIME / PENALTY_SHOOTOUT matches: fullTime may contain the ET or
//     shootout score. The API populates regularTime with the 90-min score for
//     these cases; if it's missing we return nil so the DB retains whatever
//     regulation score was stored during live sync rather than overwriting it
//     with the shootout score.
func regulationScore(am footballdata.Match) (*int, *int) {
	if am.Score.RegularTime.Home != nil {
		return am.Score.RegularTime.Home, am.Score.RegularTime.Away
	}
	if am.Score.Duration == "PENALTY_SHOOTOUT" || am.Score.Duration == "EXTRA_TIME" {
		return nil, nil
	}
	return am.Score.FullTime.Home, am.Score.FullTime.Away
}

func convertMatches(apiMatches []footballdata.Match) ([]model.Match, error) {
	matches := make([]model.Match, 0, len(apiMatches))
	for _, am := range apiMatches {
		matchDate, err := time.Parse(time.RFC3339, am.UTCDate)
		if err != nil {
			return nil, fmt.Errorf("parse match date %q: %w", am.UTCDate, err)
		}

		home, away := regulationScore(am)
		matches = append(matches, model.Match{
			ExternalID: am.ID,
			HomeTeam:   am.HomeTeam.Name,
			AwayTeam:   am.AwayTeam.Name,
			MatchDate:  matchDate.UTC(),
			Status:     model.MatchStatus(am.Status),
			HomeScore:  home,
			AwayScore:  away,
			Stage:      am.Stage,
			Group:      am.Group,
			Matchday:   am.Matchday,
		})
	}
	return matches, nil
}
