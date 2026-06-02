package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/bot"
	"halal-bet/internal/client/footballdata"
	"halal-bet/internal/client/sofascore"
	"halal-bet/internal/repository"
	"halal-bet/internal/service"
)

func main() {
	db, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	b, err := tele.NewBot(tele.Settings{
		Token:  os.Getenv("TELEGRAM_TOKEN"),
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

	fdClient := footballdata.New(os.Getenv("FOOTBALL_DATA_ORG_KEY"))
	ssClient := sofascore.New(os.Getenv("SOFASCORE_RAPIDAPI_KEY"))
	syncSvc := service.NewSyncService(fdClient, ssClient, matches, predictions)
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
	r.Post("/sync/cl-final", func(w http.ResponseWriter, req *http.Request) {
		n, err := syncSvc.SyncCLFinal(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "synced %d matches\n", n)
	})
	r.Post("/sync/events", func(w http.ResponseWriter, req *http.Request) {
		dateStr := req.URL.Query().Get("date")
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
		n, err := syncSvc.SyncMatchEvents(req.Context(), date)
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

