package repository_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"halal-bet/internal/model"
	"halal-bet/internal/repository"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration tests")
	}

	// Run migrations via goose.
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "../../db/migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	db.Close()

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(func() {
		truncate(t, pool)
		pool.Close()
	})
	truncate(t, pool) // clean slate before each test
	return pool
}

func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`TRUNCATE group_members, predictions, groups, wc2026_matches, users RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// ---- User repository ----

func TestUserRepository_CreateAndGet(t *testing.T) {
	pool := setupDB(t)
	repo := repository.NewUserRepository(pool)
	ctx := context.Background()

	user := &model.User{
		TelegramID: 100,
		Username:   "testuser",
		CreatedAt:  time.Now().UTC(),
	}

	if err := repo.CreateIfNotExist(ctx, user); err != nil {
		t.Fatalf("create: %v", err)
	}

	id, err := repo.GetIDByTelegramID(ctx, 100)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID from GetIDByTelegramID")
	}
}

func TestUserRepository_CreateIfNotExist_Idempotent(t *testing.T) {
	pool := setupDB(t)
	repo := repository.NewUserRepository(pool)
	ctx := context.Background()

	u := &model.User{TelegramID: 200, Username: "dup", CreatedAt: time.Now().UTC()}
	if err := repo.CreateIfNotExist(ctx, u); err != nil {
		t.Fatal(err)
	}
	firstID := u.ID

	u2 := &model.User{TelegramID: 200, Username: "dup2", CreatedAt: time.Now().UTC()}
	if err := repo.CreateIfNotExist(ctx, u2); err != nil {
		t.Fatal(err)
	}
	if u2.ID != firstID {
		t.Errorf("idempotent create returned different ID: %d vs %d", firstID, u2.ID)
	}
}

// ---- Match repository ----

func makeMatch(extID int64, status model.MatchStatus, home, away string) model.Match {
	return model.Match{
		ExternalID: extID,
		HomeTeam:   home,
		AwayTeam:   away,
		MatchDate:  time.Now().UTC().Add(time.Hour),
		Status:     status,
		Stage:      "GROUP_STAGE",
	}
}

func TestMatchRepository_UpsertAndGet(t *testing.T) {
	pool := setupDB(t)
	repo := repository.NewMatchRepository(pool)
	ctx := context.Background()

	m := makeMatch(1001, model.MatchStatusTimed, "Brazil", "Mexico")
	if err := repo.Upsert(ctx, []model.Match{m}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Update status to FINISHED.
	score := 2
	m.Status = model.MatchStatusFinished
	m.HomeScore = &score
	if err := repo.Upsert(ctx, []model.Match{m}); err != nil {
		t.Fatalf("upsert update: %v", err)
	}

	got, err := repo.GetUpcoming(ctx,
		time.Now().UTC().Add(-time.Hour),
		time.Now().UTC().Add(2*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Status != model.MatchStatusFinished {
		t.Errorf("expected FINISHED, got %s", got[0].Status)
	}
}

func TestMatchRepository_GetInPlay(t *testing.T) {
	pool := setupDB(t)
	repo := repository.NewMatchRepository(pool)
	ctx := context.Background()

	timed := makeMatch(2001, model.MatchStatusTimed, "A", "B")
	inPlay := makeMatch(2002, model.MatchStatusInPlay, "C", "D")
	finished := makeMatch(2003, model.MatchStatusFinished, "E", "F")

	if err := repo.Upsert(ctx, []model.Match{timed, inPlay, finished}); err != nil {
		t.Fatal(err)
	}

	results, err := repo.GetInPlay(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 in-play match, got %d", len(results))
	}
	if results[0].HomeTeam != "C" {
		t.Errorf("unexpected in-play team: %s", results[0].HomeTeam)
	}
}

// ---- Prediction repository ----

func seedUserAndMatch(t *testing.T, pool *pgxpool.Pool) (userID, matchID int64) {
	t.Helper()
	ctx := context.Background()

	uRepo := repository.NewUserRepository(pool)
	u := &model.User{TelegramID: 999, Username: "player", CreatedAt: time.Now().UTC()}
	if err := uRepo.CreateIfNotExist(ctx, u); err != nil {
		t.Fatal(err)
	}
	uid, err := uRepo.GetIDByTelegramID(ctx, 999)
	if err != nil {
		t.Fatalf("get user id: %v", err)
	}

	mRepo := repository.NewMatchRepository(pool)
	m := makeMatch(9001, model.MatchStatusTimed, "Home", "Away")
	if err := mRepo.Upsert(ctx, []model.Match{m}); err != nil {
		t.Fatal(err)
	}
	all, _ := mRepo.GetUpcoming(ctx, time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(2*time.Hour))
	return uid, all[0].ID
}

func TestPredictionRepository_UpsertAndGet(t *testing.T) {
	pool := setupDB(t)
	ctx := context.Background()
	userID, matchID := seedUserAndMatch(t, pool)
	repo := repository.NewPredictionRepository(pool)

	p := &model.Prediction{
		UserID:    userID,
		MatchID:   matchID,
		BetType:   model.BetTypeExact,
		HomeScore: 2,
		AwayScore: 1,
	}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if p.ID == 0 {
		t.Error("expected prediction ID to be set")
	}

	got, err := repo.GetByMatch(ctx, matchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(got))
	}
	if got[0].HomeScore != 2 || got[0].AwayScore != 1 {
		t.Errorf("unexpected scores: %d:%d", got[0].HomeScore, got[0].AwayScore)
	}
}

func TestPredictionRepository_Upsert_Updates(t *testing.T) {
	pool := setupDB(t)
	ctx := context.Background()
	userID, matchID := seedUserAndMatch(t, pool)
	repo := repository.NewPredictionRepository(pool)

	p := &model.Prediction{UserID: userID, MatchID: matchID, BetType: model.BetTypeExact, HomeScore: 1, AwayScore: 0}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}

	// Update the same prediction.
	p2 := &model.Prediction{UserID: userID, MatchID: matchID, BetType: model.BetTypeExact, HomeScore: 3, AwayScore: 2}
	if err := repo.Upsert(ctx, p2); err != nil {
		t.Fatal(err)
	}

	all, _ := repo.GetByMatch(ctx, matchID)
	if len(all) != 1 {
		t.Fatalf("expected 1 prediction after upsert, got %d", len(all))
	}
	if all[0].HomeScore != 3 {
		t.Errorf("expected updated score 3, got %d", all[0].HomeScore)
	}
}

func TestPredictionRepository_CountDoubleDowns(t *testing.T) {
	pool := setupDB(t)
	ctx := context.Background()
	userID, matchID := seedUserAndMatch(t, pool)
	repo := repository.NewPredictionRepository(pool)

	p := &model.Prediction{UserID: userID, MatchID: matchID, BetType: model.BetTypeExact, DoubleDown: true}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}

	count, err := repo.CountDoubleDowns(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 double down, got %d", count)
	}
}

func TestPredictionRepository_UpdatePoints(t *testing.T) {
	pool := setupDB(t)
	ctx := context.Background()
	userID, matchID := seedUserAndMatch(t, pool)
	repo := repository.NewPredictionRepository(pool)

	p := &model.Prediction{UserID: userID, MatchID: matchID, BetType: model.BetTypeExact, HomeScore: 2, AwayScore: 1}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}

	points := map[int64]int{p.ID: 5}
	if err := repo.UpdatePoints(ctx, matchID, points); err != nil {
		t.Fatal(err)
	}

	all, _ := repo.GetByMatch(ctx, matchID)
	if all[0].Points == nil || *all[0].Points != 5 {
		t.Errorf("expected points=5, got %v", all[0].Points)
	}
}

// ---- Group repository ----

func TestGroupRepository_UpsertAndLeaderboard(t *testing.T) {
	pool := setupDB(t)
	ctx := context.Background()
	userID, matchID := seedUserAndMatch(t, pool)
	_ = matchID

	gRepo := repository.NewGroupRepository(pool)
	group := &model.Group{TelegramChatID: 777, Name: "Test Group"}
	if err := gRepo.Upsert(ctx, group); err != nil {
		t.Fatalf("upsert group: %v", err)
	}
	if group.ID == 0 {
		t.Error("expected group ID to be set")
	}

	if err := gRepo.AddMember(ctx, group.ID, userID); err != nil {
		t.Fatalf("add member: %v", err)
	}
	// Duplicate add should be a no-op.
	if err := gRepo.AddMember(ctx, group.ID, userID); err != nil {
		t.Fatalf("duplicate add member: %v", err)
	}

	entries, err := gRepo.Leaderboard(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Username != "player" {
		t.Errorf("unexpected username: %q", entries[0].Username)
	}
	if entries[0].TotalPoints != 0 {
		t.Errorf("expected 0 points, got %d", entries[0].TotalPoints)
	}
}

func TestGroupRepository_GetByChatID(t *testing.T) {
	pool := setupDB(t)
	ctx := context.Background()
	repo := repository.NewGroupRepository(pool)

	group := &model.Group{TelegramChatID: 888, Name: "Chat Group"}
	if err := repo.Upsert(ctx, group); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByChatID(ctx, 888)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Chat Group" {
		t.Errorf("expected 'Chat Group', got %q", got.Name)
	}
}
