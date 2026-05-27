package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/bot"
)

func main() {
	b, err := tele.NewBot(tele.Settings{
		Token:  os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		log.Fatal(err)
	}

	h := bot.NewHandler()
	h.Register(b)

	go b.Start()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
