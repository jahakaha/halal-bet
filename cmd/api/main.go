package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"database/sql"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/bot"
	"halal-bet/internal/client/apifootball"
	"halal-bet/internal/client/footballdata"
	"halal-bet/internal/config"
	"halal-bet/internal/repository"
	"halal-bet/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(sqlDB, "db/migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	_ = sqlDB.Close()

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		log.Fatal(err)
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
	afClient := apifootball.New(cfg.ApiFootballKey)
	syncSvc := service.NewSyncService(fdClient, afClient, matches, predictions, b, adminTelegramID)
	notifSvc := service.NewNotificationService(b, groups, matches)
	lbSvc := service.NewLeaderboardService(groups, matches, predictions)
	service.StartScheduler(notifSvc, syncSvc)

	h := bot.NewHandler(users, matches, predictions, groups, tournament, lbSvc)
	h.Register(b)

	go b.Start()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/tournament/predictions", func(w http.ResponseWriter, r *http.Request) {
		predictions, err := tournament.GetSummary(r.Context())
		if err != nil {
			http.Error(w, "get tournament predictions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(predictions); err != nil {
			http.Error(w, "encode tournament predictions: "+err.Error(), http.StatusInternalServerError)
		}
	})
	r.Post("/debug/set-date", func(w http.ResponseWriter, req *http.Request) {
		dateStr := req.URL.Query().Get("date")
		if dateStr == "" {
			bot.SetTestDate(nil)
			fmt.Fprintln(w, "test date cleared")
			return
		}
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		bot.SetTestDate(&d)
		fmt.Fprintf(w, "test date set to %s\n", dateStr)
	})
	r.Post("/notify/results", func(w http.ResponseWriter, req *http.Request) {
		now, err := parseDateParam(req.URL.Query().Get("date"))
		if err != nil {
			http.Error(w, "invalid date, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		notifSvc.SendDailyResults(req.Context(), now)
		fmt.Fprintln(w, "results sent")
	})
	r.Post("/notify/matches", func(w http.ResponseWriter, req *http.Request) {
		now, err := parseDateParam(req.URL.Query().Get("date"))
		if err != nil {
			http.Error(w, "invalid date, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		notifSvc.SendDailyMatches(req.Context(), now)
		fmt.Fprintln(w, "matches sent")
	})
	r.Post("/sync/wc2026", func(w http.ResponseWriter, req *http.Request) {
		n, err := syncSvc.SyncWC2026(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "synced %d matches\n", n)
	})
	r.Post("/sync/events", func(w http.ResponseWriter, req *http.Request) {
		n, err := syncSvc.SyncMatchEvents(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "synced events for %d matches\n", n)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: ":" + port, Handler: r}

	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func parseDateParam(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	return time.Parse("2006-01-02", s)
}
