package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
	"halal-bet/internal/repository"
	"halal-bet/internal/service"
)

type Handler struct {
	users       repository.UserRepository
	matches     repository.MatchRepository
	predictions repository.PredictionRepository
	groups      repository.GroupRepository
	leaderboard *service.LeaderboardService
	store       *stateStore
}

func NewHandler(
	users repository.UserRepository,
	matches repository.MatchRepository,
	predictions repository.PredictionRepository,
	groups repository.GroupRepository,
	leaderboard *service.LeaderboardService,
) *Handler {
	return &Handler{
		users:       users,
		matches:     matches,
		predictions: predictions,
		groups:      groups,
		leaderboard: leaderboard,
		store:       newStateStore(),
	}
}

func (h *Handler) Register(b *tele.Bot) {
	b.Handle("/start", h.Start)
	b.Handle("/matches", h.Matches)
	b.Handle("/leaderboard", h.Leaderboard)
	b.Handle("/bets", h.Bets)

	b.Handle(tele.OnText, h.OnText)
	b.Handle(tele.OnCallback, h.OnCallback)
	b.Handle(tele.OnAddedToGroup, h.OnAddedToGroup)

	_ = b.SetCommands([]tele.Command{
		{Text: "matches", Description: "Матчи сегодня"},
		{Text: "bets", Description: "Ставки на сегодня"},
		{Text: "leaderboard", Description: "Таблица группы"},
	})
}

func (h *Handler) Start(c tele.Context) error {
	sender := c.Sender()
	ctx := context.Background()

	user := &model.User{
		TelegramID: sender.ID,
		Username:   sender.Username,
		CreatedAt:  time.Now().UTC(),
	}
	if err := h.users.CreateIfNotExist(ctx, user); err != nil {
		return err
	}
	if err := h.registerGroupMember(c); err != nil {
		return err
	}

	// Deep link: /start m_123 — сразу открываем ставку на матч
	if payload := c.Message().Payload; strings.HasPrefix(payload, "m_") {
		idStr := strings.TrimPrefix(payload, "m_")
		return h.openMatchBet(c, idStr)
	}

	return c.Send(fmt.Sprintf("Привет, %s! Добро пожаловать в HalalBet", sender.FirstName))
}

func (h *Handler) OnAddedToGroup(c tele.Context) error {
	chat := c.Chat()
	group := &model.Group{
		TelegramChatID: chat.ID,
		Name:           chat.Title,
	}
	return h.groups.Upsert(context.Background(), group)
}

// openMatchBet устанавливает состояние и показывает выбор типа ставки.
func (h *Handler) openMatchBet(c tele.Context, idStr string) error {
	matchID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Send("Неверная ссылка на матч.")
	}

	ctx := context.Background()
	m, err := h.matches.GetByID(ctx, matchID)
	if err != nil {
		return c.Send("Матч не найден.")
	}
	if time.Until(m.MatchDate) < 5*time.Minute {
		return c.Send("Ставки на этот матч уже закрыты.")
	}

	msg := buildMatchBetMsg(context.Background(), h, m)
	kb := &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{{Text: "Точный счёт +5", Data: "bt|exact"}},
			{{Text: "Разница голов +3", Data: "bt|diff"}},
			{{Text: "Исход +1", Data: "bt|outcome"}},
		},
	}
	sent, err := c.Bot().Send(c.Recipient(), msg, kb, tele.ModeMarkdown)
	if err != nil {
		return err
	}
	h.store.set(c.Sender().ID, &predictionState{
		matchID:  matchID,
		homeTeam: m.HomeTeam,
		awayTeam: m.AwayTeam,
		msgID:    sent.ID,
	})
	return nil
}

// registerGroupMember авто-добавляет пользователя в группу если сообщение из группового чата.
func (h *Handler) registerGroupMember(c tele.Context) error {
	chat := c.Chat()
	if chat.Type == tele.ChatPrivate {
		return nil
	}

	ctx := context.Background()
	group := &model.Group{
		TelegramChatID: chat.ID,
		Name:           chat.Title,
	}
	if err := h.groups.Upsert(ctx, group); err != nil {
		return err
	}

	userID, err := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		return err
	}
	return h.groups.AddMember(ctx, group.ID, userID)
}
