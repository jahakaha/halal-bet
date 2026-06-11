package service

import (
	"context"
	"testing"
	"time"

	"halal-bet/internal/model"
)

// --- mock repositories ---

type mockMatchRepo struct {
	matches  []model.Match
	upserted []model.Match
	inPlay   []model.Match
	pending  []model.Match
}

func (m *mockMatchRepo) Upsert(_ context.Context, matches []model.Match) error {
	m.upserted = append(m.upserted, matches...)
	return nil
}
func (m *mockMatchRepo) GetByID(_ context.Context, id int64) (*model.Match, error) {
	for _, match := range m.matches {
		if match.ID == id {
			return &match, nil
		}
	}
	return nil, nil
}
func (m *mockMatchRepo) GetUpcoming(_ context.Context, _, _ time.Time) ([]model.Match, error) {
	return m.matches, nil
}
func (m *mockMatchRepo) GetFinishedWithPendingScores(_ context.Context) ([]model.Match, error) {
	return m.pending, nil
}
func (m *mockMatchRepo) GetFinishedForEventSync(_ context.Context) ([]model.Match, error) {
	return nil, nil
}
func (m *mockMatchRepo) UpdateEvents(_ context.Context, _, _ int64, _, _, _ bool) error {
	return nil
}
func (m *mockMatchRepo) GetGroupStandings(_ context.Context, _ string) ([]model.StandingEntry, error) {
	return nil, nil
}
func (m *mockMatchRepo) GetFinishedInWindow(_ context.Context, _, _ time.Time) ([]model.Match, error) {
	return nil, nil
}
func (m *mockMatchRepo) GetInPlay(_ context.Context) ([]model.Match, error) {
	return m.inPlay, nil
}
func (m *mockMatchRepo) UpdateStatus(_ context.Context, _ int64, _ model.MatchStatus, _, _ *int) error {
	return nil
}

type mockPredictionRepo struct {
	byMatch  map[int64][]model.Prediction
	updated  map[int64]int // predID → points
}

func newMockPredictionRepo() *mockPredictionRepo {
	return &mockPredictionRepo{
		byMatch: make(map[int64][]model.Prediction),
		updated: make(map[int64]int),
	}
}

func (r *mockPredictionRepo) Upsert(_ context.Context, _ *model.Prediction) error { return nil }
func (r *mockPredictionRepo) GetByUserAndMatch(_ context.Context, _, _ int64) (*model.Prediction, error) {
	return nil, nil
}
func (r *mockPredictionRepo) GetByMatch(_ context.Context, matchID int64) ([]model.Prediction, error) {
	return r.byMatch[matchID], nil
}
func (r *mockPredictionRepo) GetByMatchWithUsers(_ context.Context, matchID int64) ([]model.PredictionWithUser, error) {
	preds := r.byMatch[matchID]
	result := make([]model.PredictionWithUser, len(preds))
	for i, p := range preds {
		result[i] = model.PredictionWithUser{Prediction: p}
	}
	return result, nil
}
func (r *mockPredictionRepo) CountDoubleDowns(_ context.Context, _, _ int64) (int, error) {
	return 0, nil
}
func (r *mockPredictionRepo) GetByMatchWithUsersInGroup(_ context.Context, _, _ int64) ([]model.PredictionWithUser, error) {
	return nil, nil
}
func (r *mockPredictionRepo) UpdatePoints(_ context.Context, _ int64, points map[int64]int) error {
	for id, pts := range points {
		r.updated[id] = pts
	}
	return nil
}

func intPtr(i int) *int { return &i }

// --- tests ---

func TestFinalizeFinishedMatches_ExactBet(t *testing.T) {
	home, away := 2, 1
	m := model.Match{
		ID:        1,
		Status:    model.MatchStatusFinished,
		HomeScore: intPtr(home),
		AwayScore: intPtr(away),
	}

	pred := model.Prediction{
		ID:        10,
		MatchID:   1,
		BetType:   model.BetTypeExact,
		HomeScore: 2,
		AwayScore: 1,
	}

	matchRepo := &mockMatchRepo{pending: []model.Match{m}}
	predRepo := newMockPredictionRepo()
	predRepo.byMatch[1] = []model.Prediction{pred}

	svc := &SyncService{matches: matchRepo, predictions: predRepo}
	if err := svc.FinalizeFinishedMatches(context.Background()); err != nil {
		t.Fatal(err)
	}

	pts, ok := predRepo.updated[10]
	if !ok {
		t.Fatal("prediction 10 was not updated")
	}
	if pts != 5 {
		t.Errorf("exact match should give 5 pts, got %d", pts)
	}
}

func TestFinalizeFinishedMatches_SkipsAlreadyScored(t *testing.T) {
	scored := 3
	pred := model.Prediction{
		ID:        20,
		MatchID:   1,
		BetType:   model.BetTypeExact,
		HomeScore: 1,
		AwayScore: 0,
		Points:    &scored,
	}

	m := model.Match{
		ID:        1,
		Status:    model.MatchStatusFinished,
		HomeScore: intPtr(2),
		AwayScore: intPtr(1),
	}

	matchRepo := &mockMatchRepo{pending: []model.Match{m}}
	predRepo := newMockPredictionRepo()
	predRepo.byMatch[1] = []model.Prediction{pred}

	svc := &SyncService{matches: matchRepo, predictions: predRepo}
	if err := svc.FinalizeFinishedMatches(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, ok := predRepo.updated[20]; ok {
		t.Error("already scored prediction should not be updated")
	}
}

func TestFinalizeFinishedMatches_NoPendingMatches(t *testing.T) {
	matchRepo := &mockMatchRepo{pending: nil}
	predRepo := newMockPredictionRepo()

	svc := &SyncService{matches: matchRepo, predictions: predRepo}
	if err := svc.FinalizeFinishedMatches(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(predRepo.updated) != 0 {
		t.Error("no updates expected when no pending matches")
	}
}

func TestFinalizeFinishedMatches_DoubleDown(t *testing.T) {
	m := model.Match{
		ID:        1,
		Status:    model.MatchStatusFinished,
		HomeScore: intPtr(2),
		AwayScore: intPtr(1),
	}
	pred := model.Prediction{
		ID:         30,
		MatchID:    1,
		BetType:    model.BetTypeExact,
		HomeScore:  2,
		AwayScore:  1,
		DoubleDown: true,
	}

	matchRepo := &mockMatchRepo{pending: []model.Match{m}}
	predRepo := newMockPredictionRepo()
	predRepo.byMatch[1] = []model.Prediction{pred}

	svc := &SyncService{matches: matchRepo, predictions: predRepo}
	if err := svc.FinalizeFinishedMatches(context.Background()); err != nil {
		t.Fatal(err)
	}

	if pts := predRepo.updated[30]; pts != 10 {
		t.Errorf("DD exact should give 10 pts, got %d", pts)
	}
}

func TestQuietWindowSleep(t *testing.T) {
	// Just verify it returns 0 outside quiet window and >0 inside.
	// We can't control time.Now(), so we test the logic indirectly via boundaries.
	// This is a sanity check that the function exists and compiles.
	_ = quietWindowSleep()
}

func TestNameMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Paris Saint-Germain", "Paris Saint-Germain FC", true},
		{"Arsenal FC", "Arsenal", true},
		{"Manchester City", "Chelsea", false},
		{"PSG", "Paris Saint-Germain", false}, // abbreviation doesn't match
	}
	for _, tc := range cases {
		got := nameMatch(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("nameMatch(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
