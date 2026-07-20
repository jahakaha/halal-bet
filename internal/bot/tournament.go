package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

// GroupStageDeadline — последний день группового этапа ЧМ26.
// После этой даты предсказания закрыты для всех.
var GroupStageDeadline = time.Date(2026, 7, 2, 0, 0, 0, 0, almatyLoc)

// personalDeadline возвращает персональный дедлайн юзера:
// 3 дня с момента регистрации, но не позже конца группового этапа.
func personalDeadline(registeredAt time.Time) time.Time {
	d := registeredAt.Add(72 * time.Hour)
	if d.After(GroupStageDeadline) {
		return GroupStageDeadline
	}
	return d
}

func (h *Handler) Predict(c tele.Context) error {
	if c.Chat().Type != tele.ChatPrivate {
		return c.Send("Ставки на турнир делаются в личных сообщениях с ботом.")
	}

	ctx := context.Background()
	user, err := h.users.GetByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		return err
	}

	preds, _ := h.tournament.GetByUser(ctx, user.ID)
	now := time.Now()

	if now.After(GroupStageDeadline) {
		return c.Send(formatTournamentResults(preds), tele.ModeMarkdown)
	}

	deadline := personalDeadline(user.CreatedAt)
	if now.After(deadline) {
		return c.Send(formatTournamentResults(preds), tele.ModeMarkdown)
	}

	return c.Send(formatTournamentMenu(preds, deadline), &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{{Text: "🏆 Победитель ЧМ26", Data: "tp|" + model.TournamentBetWinner}},
			{{Text: "⚽️ Лучший бомбардир", Data: "tp|" + model.TournamentBetTopScorer}},
		},
	}, tele.ModeMarkdown)
}

// TournamentResults shows tournament predictions and their earned points for
// members of the current Telegram group.
func (h *Handler) TournamentResults(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return c.Send("Результаты доступны только в групповом чате.")
	}

	_ = h.registerGroupMember(c)
	ctx := context.Background()
	group, err := h.groups.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		return c.Send("Группа не зарегистрирована. Напишите /start сначала.")
	}

	predictions, err := h.tournament.GetSummary(ctx, group.ID)
	if err != nil {
		return err
	}
	return c.Send(formatTournamentGroupResults(predictions), tele.ModeMarkdown)
}

func (h *Handler) handleTournamentBet(c tele.Context, betType string) error {
	ctx := context.Background()
	user, err := h.users.GetByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Пользователь не найден"})
	}
	now := time.Now()
	if now.After(GroupStageDeadline) || now.After(personalDeadline(user.CreatedAt)) {
		return c.Respond(&tele.CallbackResponse{Text: "Приём ставок закрыт"})
	}

	label := map[string]string{
		model.TournamentBetWinner:    "победителя ЧМ26 (название команды)",
		model.TournamentBetTopScorer: "лучшего бомбардира (имя игрока)",
	}[betType]

	_ = c.Respond()
	h.store.setTournament(c.Sender().ID, betType)
	return c.Send(fmt.Sprintf("Введи %s:", label))
}

func (h *Handler) handleTournamentText(c tele.Context, betType, text string) error {
	ctx := context.Background()
	userID, err := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		return err
	}

	if err := h.tournament.Upsert(ctx, userID, betType, text); err != nil {
		return err
	}

	label := map[string]string{
		model.TournamentBetWinner:    "🏆 Победитель",
		model.TournamentBetTopScorer: "⚽️ Бомбардир",
	}[betType]

	h.store.clearTournament(c.Sender().ID)
	return c.Send(fmt.Sprintf("Принято! %s → *%s*", label, text), tele.ModeMarkdown)
}

func formatTournamentMenu(preds []model.TournamentPrediction, deadline time.Time) string {
	byType := map[string]string{}
	for _, p := range preds {
		byType[p.Type] = p.Value
	}

	var sb strings.Builder
	sb.WriteString("🔮 *Предсказания на ЧМ26*\n\n")

	if v, ok := byType[model.TournamentBetWinner]; ok {
		sb.WriteString(fmt.Sprintf("🏆 Победитель: *%s*\n", v))
	} else {
		sb.WriteString("🏆 Победитель: _не указан_\n")
	}

	if v, ok := byType[model.TournamentBetTopScorer]; ok {
		sb.WriteString(fmt.Sprintf("⚽️ Бомбардир: *%s*\n", v))
	} else {
		sb.WriteString("⚽️ Бомбардир: _не указан_\n")
	}

	d := deadline.In(almatyLoc)
	sb.WriteString(fmt.Sprintf("\n_Можно изменить до %d %s %02d:%02d_",
		d.Day(), monthName(d.Month()), d.Hour(), d.Minute()))
	return sb.String()
}

func monthName(m time.Month) string {
	months := []string{"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря"}
	return months[m]
}

func formatTournamentResults(preds []model.TournamentPrediction) string {
	if len(preds) == 0 {
		return "🔮 Ты не сделал предсказания на ЧМ26."
	}

	byType := map[string]string{}
	for _, p := range preds {
		byType[p.Type] = p.Value
	}

	var sb strings.Builder
	sb.WriteString("🔮 *Твои предсказания на ЧМ26*\n\n")
	if v, ok := byType[model.TournamentBetWinner]; ok {
		sb.WriteString(fmt.Sprintf("🏆 Победитель: *%s*\n", v))
	}
	if v, ok := byType[model.TournamentBetTopScorer]; ok {
		sb.WriteString(fmt.Sprintf("⚽️ Бомбардир: *%s*\n", v))
	}
	sb.WriteString("\n_Приём ставок закрыт_")
	return sb.String()
}

func formatTournamentGroupResults(preds []model.TournamentPredictionSummary) string {
	if len(preds) == 0 {
		return "🔮 В этой группе пока нет ставок на турнир."
	}

	var sb strings.Builder
	sb.WriteString("🔮 *Ставки на ЧМ26*\n\n")
	for _, p := range preds {
		name := p.Username
		if name == "" {
			name = "Аноним"
		}
		sb.WriteString(fmt.Sprintf("*%s* — %d %s\n🏆 %s\n⚽️ %s\n\n",
			name, p.Points, pointWord(p.Points), p.Team, p.TopScorer))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}
