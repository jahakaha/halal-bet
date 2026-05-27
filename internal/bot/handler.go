package bot

import (
	"context"
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
	"halal-bet/internal/repository"
)

type Handler struct {
	users repository.UserRepository
}

func NewHandler(users repository.UserRepository) *Handler {
	return &Handler{users: users}
}

func (h *Handler) Register(b *tele.Bot) {
	b.Handle("/start", h.Start)
}

func (h *Handler) Start(c tele.Context) error {
	sender := c.Sender()

	user := &model.User{
		TelegramID: sender.ID,
		Username:   sender.Username,
		CreatedAt:  time.Now(),
	}

	if err := h.users.CreateIfNotExist(context.Background(), user); err != nil {
		return err
	}

	return c.Send(fmt.Sprintf("Привет, %s! Добро пожаловать в HalalBet 🎯", sender.FirstName))
}
