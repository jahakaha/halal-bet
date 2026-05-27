package bot

import (
	tele "gopkg.in/telebot.v3"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Register(b *tele.Bot) {
	b.Handle("/start", h.Start)
}

func (h *Handler) Start(c tele.Context) error {
	return c.Send("Hello World")
}
