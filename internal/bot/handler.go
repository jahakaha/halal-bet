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

// isNotModified returns true for the Telegram "message is not modified" error.
// Treat as success — content is already correct.
func isNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "message is not modified")
}

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
	b.Handle("/tournament_results", h.TournamentResults)
	b.Handle("/me", h.Me)
	b.Handle("/stats", h.Stats)

	b.Handle(tele.OnText, h.OnText)
	b.Handle(tele.OnCallback, h.OnCallback)
	b.Handle(tele.OnAddedToGroup, h.OnAddedToGroup)

	_ = b.SetCommands([]tele.Command{
		{Text: "matches", Description: "Матчи сегодня"},
		{Text: "bets", Description: "Ставки дня"},
		{Text: "leaderboard", Description: "Таблица тотализатора"},
		{Text: "me", Description: "Моя история ставок"},
		{Text: "groups", Description: "Таблица групп ЧМ26"},
		{Text: "predict", Description: "Предсказания на ЧМ26"},
		{Text: "tournament_results", Description: "Ставки на турнир"},
		{Text: "stats", Description: "Статистика группы"},
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

	// Deep link: /start m_123 or m_123_c_{chatID}
	if payload := c.Message().Payload; strings.HasPrefix(payload, "m_") {
		parts := strings.SplitN(strings.TrimPrefix(payload, "m_"), "_c_", 2)
		var chatID int64
		if len(parts) == 2 {
			chatID, _ = strconv.ParseInt(parts[1], 10, 64)
		}
		return h.openMatchBet(c, parts[0], chatID)
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
// chatID — Telegram chat ID группы из диплинка (0 если неизвестно).
func (h *Handler) openMatchBet(c tele.Context, idStr string, chatID int64) error {
	matchID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Send("Неверная ссылка на матч.")
	}

	ctx := context.Background()
	m, err := h.matches.GetByID(ctx, matchID)
	if err != nil {
		return c.Send("Матч не найден.")
	}

	canEdit := time.Until(m.MatchDate) > 5*time.Minute && m.Status == model.MatchStatusTimed

	userID, dbErr := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if dbErr == nil {
		if existing, predErr := h.predictions.GetByUserAndMatch(ctx, userID, matchID); predErr == nil {
			block := groupStandingsBlock(ctx, h, m)
			return c.Send(formatExistingBet(block, m, existing, canEdit), buildExistingBetKeyboard(m.ID, canEdit), tele.ModeMarkdown)
		}
	}

	if !canEdit {
		return c.Send("Ставки на этот матч уже закрыты.")
	}

	msg := buildMatchBetMsg(ctx, h, m)
	sent, err := c.Bot().Send(c.Recipient(), msg, tele.ModeMarkdown)
	if err != nil {
		return err
	}
	h.store.set(c.Sender().ID, &predictionState{
		matchID:     matchID,
		groupChatID: chatID,
		homeTeam:    m.HomeTeam,
		awayTeam:    m.AwayTeam,
		betType:     model.BetTypeExact,
		msgID:       sent.ID,
	})
	return nil
}

func (h *Handler) Description(c tele.Context) error {
	return c.Send(`*HalalBet — правила игры*

Каждый день перед матчами бот присылает расписание в групповой чат. Открой личку с ботом, выбери матч и сделай ставку. После финального свистка очки начисляются автоматически.

*Ставка на счёт*

Вводишь точный счёт матча. Система сама определяет сколько ты угадал и начисляет лучшее из возможного:

🥇 Угадал точный счёт — *+5 очков*
🥈 Угадал разницу голов — *+3 очка*
🥉 Угадал исход (победа/ничья) — *+1 очко*
❌ Не угадал ничего — 0 очков

_Пример: поставил 3:1. Сыграли 2:0 — разница одинаковая (+2), получаешь +3. Сыграли 1:0 — хозяева выиграли, получаешь +1._

*Дополнительные ставки*

К основной ставке можно добавить одну из трёх рискованных ставок на события матча:

🥅 Будет пенальти — *+2* если да / *−1* если нет
🟥 Будет красная карточка — *+3* если да / *−2* если нет
🤦 Будет автогол — *+5* если да / *−3* если нет

_Доп. ставки не зависят от основного результата — очки/штраф начисляются отдельно._

*Double Down 🔥*

Перед сохранением можно включить Double Down — это удваивает очки за основную ставку.

🔥 Угадал → очки x2 (+10 за точный счёт, +6 за разницу, +2 за исход)
💀 Не угадал ничего → *−2 очка*

Лимит: *5 раз* за весь турнир. Используй с умом.

*Расписание пушей*

📊 *12:00* — результаты вчерашних матчей и таблица очков.
📅 *12:05* — матчи следующего дня. С этого момента открываются ставки.

Ставки закрываются за *5 минут* до начала матча.

*Предсказания на весь турнир* — /predict

Разовые ставки до старта ЧМ:
🏆 Чемпион ЧМ 2026 — *+20 очков*
⚽️ Лучший бомбардир — *+15 очков*

*Команды*

/matches — матчи завтра
/bets — ставки текущего дня
/leaderboard — таблица очков (с живыми очками во время матча)
/groups — таблица групп ЧМ 2026
/predict — предсказания на турнир`, tele.ModeMarkdown)
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

	sender := c.Sender()
	if err := h.users.CreateIfNotExist(ctx, &model.User{
		TelegramID: sender.ID,
		Username:   sender.Username,
	}); err != nil {
		return err
	}

	userID, err := h.users.GetIDByTelegramID(ctx, sender.ID)
	if err != nil {
		return err
	}
	return h.groups.AddMember(ctx, group.ID, userID)
}
