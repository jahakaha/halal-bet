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
	tournament  repository.TournamentRepository
	leaderboard *service.LeaderboardService
	store       *stateStore
}

func NewHandler(
	users repository.UserRepository,
	matches repository.MatchRepository,
	predictions repository.PredictionRepository,
	groups repository.GroupRepository,
	tournament repository.TournamentRepository,
	leaderboard *service.LeaderboardService,
) *Handler {
	return &Handler{
		users:       users,
		matches:     matches,
		predictions: predictions,
		groups:      groups,
		tournament:  tournament,
		leaderboard: leaderboard,
		store:       newStateStore(),
	}
}

func (h *Handler) Register(b *tele.Bot) {
	b.Handle("/start", h.Start)
	b.Handle("/description", h.Description)
	b.Handle("/matches", h.Matches)
	b.Handle("/leaderboard", h.Leaderboard)
	b.Handle("/bets", h.Bets)
	b.Handle("/groups", h.Groups)
	b.Handle("/predict", h.Predict)

	b.Handle(tele.OnText, h.OnText)
	b.Handle(tele.OnCallback, h.OnCallback)
	b.Handle(tele.OnAddedToGroup, h.OnAddedToGroup)

	_ = b.SetCommands([]tele.Command{
		{Text: "matches", Description: "Матчи сегодня"},
		{Text: "bets", Description: "Ставки дня"},
		{Text: "leaderboard", Description: "Таблица тотализатора"},
		{Text: "groups", Description: "Таблица групп ЧМ26"},
		{Text: "predict", Description: "Предсказания на ЧМ26"},
		{Text: "description", Description: "Правила игры"},
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

	name := sender.Username
	if name == "" {
		name = sender.FirstName
	} else {
		name = "@" + name
	}
	return c.Send(fmt.Sprintf("Ассаламу алейкум, %s!\n\nДобро пожаловать в *HalalBet* — тотализатор ЧМ 2026.\n\nПравила и система очков: /description", name), tele.ModeMarkdown)
}

func (h *Handler) OnAddedToGroup(c tele.Context) error {
	chat := c.Chat()
	group := &model.Group{
		TelegramChatID: chat.ID,
		Name:           chat.Title,
	}
	return h.groups.Upsert(context.Background(), group)
}

// openMatchBet устанавливает состояние и показывает ставку или выбор типа.
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

	canEdit := time.Until(m.MatchDate) > 5*time.Minute

	userID, dbErr := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if dbErr == nil {
		if existing, predErr := h.predictions.GetByUserAndMatch(ctx, userID, matchID); predErr == nil {
			return c.Send(formatExistingBet(m, existing, canEdit), buildExistingBetKeyboard(m.ID, canEdit), tele.ModeMarkdown)
		}
	}

	if !canEdit {
		return c.Send("Ставки на этот матч уже закрыты.")
	}

	msg := buildMatchBetMsg(ctx, h, m)
	sent, err := c.Bot().Send(c.Recipient(), msg, betTypeKeyboard(), tele.ModeMarkdown)
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

func (h *Handler) Description(c tele.Context) error {
	return c.Send(`*HalalBet — правила игры*

Делай ставки на матчи ЧМ 2026 и соревнуйся с друзьями.

*Основная ставка*
🎯 Точный счёт — +5 очков
⚖️ Разница голов — +3 очка
🏆 Исход матча — +1 очко

*Дополнительные ставки:*
🥅 Пенальти в матче — +2 / −1
🟥 Красная карточка — +3 / −2
🤦 Автогол — +5 / −3

*Double Down 🔥*
Удваивает очки за правильный прогноз основной ставки.
При неправильном — −1 очко.
Лимит: 5 использований за турнир.

⏰ Ставки закрываются за 5 минут до начала матча.

*Предсказания на турнир:* /predict
🏆 Чемпион ЧМ 2026 — +20 очков
⚽️ Лучший бомбардир — +15 очков
Дедлайн: 11 июня 23:30 (Алматы)

/matches · /leaderboard · /bets · /groups`, tele.ModeMarkdown)
}

func (h *Handler) sendPrivateOnlyHint(c tele.Context) error {
	user := &tele.User{ID: c.Sender().ID}
	_, err := c.Bot().Send(user, "Используй эту команду в нашем личном чате.")
	if err == nil {
		return nil
	}
	kb := &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{{
			{Text: "Открыть личку", URL: "https://t.me/" + c.Bot().Me.Username},
		}},
	}
	return c.Send("Напиши мне в личку!", kb)
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
