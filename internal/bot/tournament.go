package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

var TournamentDeadline = time.Date(2026, 6, 11, 23, 30, 0, 0, almatyLoc)

func (h *Handler) Predict(c tele.Context) error {
	if c.Chat().Type != tele.ChatPrivate {
		return c.Send("Ставки на турнир делаются в личных сообщениях с ботом.")
	}

	ctx := context.Background()
	userID, err := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		return err
	}

	preds, _ := h.tournament.GetByUser(ctx, userID)

	if time.Now().After(TournamentDeadline) {
		return c.Send(formatTournamentResults(preds), tele.ModeMarkdown)
	}

	return c.Send(formatTournamentMenu(preds), &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{{Text: "🏆 Победитель ЧМ26", Data: "tp|" + model.TournamentBetWinner}},
			{{Text: "⚽️ Лучший бомбардир", Data: "tp|" + model.TournamentBetTopScorer}},
		},
	}, tele.ModeMarkdown)
}

func (h *Handler) handleTournamentBet(c tele.Context, betType string) error {
	if time.Now().After(TournamentDeadline) {
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

func formatTournamentMenu(preds []model.TournamentPrediction) string {
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

	sb.WriteString("\nМожно изменить до 11 июня 23:30:")
	return sb.String()
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
