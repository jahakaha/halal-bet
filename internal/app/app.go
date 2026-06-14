package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/bot"
	"halal-bet/internal/client/footballdata"
	"halal-bet/internal/client/sofascore"
	"halal-bet/internal/config"
	"halal-bet/internal/handler"
	"halal-bet/internal/repository"
	"halal-bet/internal/service"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	if err := goose.Up(sqlDB, "db/migrations"); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	_ = sqlDB.Close()

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer db.Close()

	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		return fmt.Errorf("telegram bot: %w", err)
	}

	users := repository.NewUserRepository(db)
	matches := repository.NewMatchRepository(db)
	predictions := repository.NewPredictionRepository(db)
	groups := repository.NewGroupRepository(db)
	tournament := repository.NewTournamentRepository(db)

	adminTelegramID, err := users.GetTelegramIDByUsername(context.Background(), "jomirzak")
	if err != nil {
		log.Printf("warn: admin user not found in db: %v", err)
	}

	fdClient := footballdata.New(cfg.FootballDataKey)
	ssClient := sofascore.New(cfg.SofascoreKey)
	syncSvc := service.NewSyncService(fdClient, ssClient, matches, predictions, b, adminTelegramID)
	notifSvc := service.NewNotificationService(b, groups, matches)
	lbSvc := service.NewLeaderboardService(groups, matches, predictions)
	service.StartScheduler(notifSvc, syncSvc)

	h := bot.NewHandler(users, matches, predictions, groups, tournament, lbSvc)
	h.Register(b)

	go b.Start()

	r := chi.NewRouter()
	handler.New(syncSvc).Register(r)

	log.Printf("listening on :%s", cfg.Port)
	return http.ListenAndServe(":"+cfg.Port, r)
}
