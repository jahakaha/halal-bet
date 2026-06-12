package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)


func (h *Handler) OnCallback(c tele.Context) error {
	data := c.Data()
	switch {
	case strings.HasPrefix(data, "tp|"):
		return h.handleTournamentBet(c, data[3:])
	case strings.HasPrefix(data, "grp|"):
		return h.handleGroupStandings(c, data[4:])
	case strings.HasPrefix(data, "m|"):
		return h.handleMatchSelect(c, data[2:])
	case strings.HasPrefix(data, "eb|"):
		return h.handleEditBet(c, data[3:])
	case data == "back|m":
		return h.handleBackToMatches(c)
	case data == "back|grp":
		return h.handleBackToGroups(c)
	case strings.HasPrefix(data, "s|"):
		return h.handleSpecialToggle(c, data[2:])
	case data == "dd":
		return h.handleDoubleDownToggle(c)
	case data == "sv":
		return h.handleSave(c)
	}
	return c.Respond()
}

func (h *Handler) OnText(c tele.Context) error {
	if c.Chat().Type != tele.ChatPrivate {
		return nil
	}

	text := strings.TrimSpace(c.Text())

	// Tournament prediction input takes priority
	if betType, ok := h.store.getTournament(c.Sender().ID); ok {
		return h.handleTournamentText(c, betType, text)
	}

	st, ok := h.store.get(c.Sender().ID)
	if !ok || st.betType == "" {
		return nil
	}

	return h.parseExactScore(c, st, text)
}

// handleMatchSelect — пользователь нажал на матч, редактируем то же сообщение.
func (h *Handler) handleMatchSelect(c tele.Context, idStr string) error {
	matchID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Respond()
	}

	ctx := context.Background()
	selected, err := h.matches.GetByID(ctx, matchID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Матч не найден"})
	}

	canEdit := time.Until(selected.MatchDate) > 5*time.Minute && selected.Status == model.MatchStatusTimed

	userID, dbErr := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if dbErr == nil {
		if existing, predErr := h.predictions.GetByUserAndMatch(ctx, userID, matchID); predErr == nil {
			block := groupStandingsBlock(ctx, h, selected)
			if err := c.Respond(); err != nil {
				return err
			}
			_, err := c.Bot().Edit(c.Message(), formatExistingBet(block, selected, existing, canEdit), buildExistingBetKeyboard(selected.ID, canEdit), tele.ModeMarkdown)
			if isNotModified(err) {
				return nil
			}
			return err
		}
	}

	if !canEdit {
		if err := c.Respond(); err != nil {
			return err
		}
		_, err := c.Bot().Edit(c.Message(),
			fmt.Sprintf("*%s — %s*\n\nСтавки закрыты.", withFlag(selected.HomeTeam), withFlag(selected.AwayTeam)),
			backToMatchesKeyboard(), tele.ModeMarkdown)
		return err
	}

	msg := buildMatchBetMsg(ctx, h, selected)
	if err := c.Respond(); err != nil {
		return err
	}
	_, err = c.Bot().Edit(c.Message(), msg, backToMatchesKeyboard(), tele.ModeMarkdown)
	if err != nil {
		return err
	}

	h.store.set(c.Sender().ID, &predictionState{
		matchID:  matchID,
		homeTeam: selected.HomeTeam,
		awayTeam: selected.AwayTeam,
		betType:  model.BetTypeExact,
		msgID:    c.Message().ID,
	})
	return nil
}

func (h *Handler) handleEditBet(c tele.Context, idStr string) error {
	matchID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Respond()
	}
	ctx := context.Background()
	m, err := h.matches.GetByID(ctx, matchID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Матч не найден"})
	}
	if time.Until(m.MatchDate) < 5*time.Minute {
		return c.Respond(&tele.CallbackResponse{Text: "Ставки уже закрыты"})
	}

	msg := buildMatchBetMsg(ctx, h, m)
	st := &predictionState{
		matchID:  matchID,
		homeTeam: m.HomeTeam,
		awayTeam: m.AwayTeam,
		betType:  model.BetTypeExact,
	}
	hasPrediction := false
	userID, dbErr := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if dbErr == nil {
		if existing, predErr := h.predictions.GetByUserAndMatch(ctx, userID, matchID); predErr == nil {
			hasPrediction = true
			st.homeScore = existing.HomeScore
			st.awayScore = existing.AwayScore
			st.doubleDown = existing.DoubleDown
			switch {
			case existing.BetPenalty:
				st.special = specialPenalty
			case existing.BetRedCard:
				st.special = specialRedCard
			case existing.BetOwnGoal:
				st.special = specialOwnGoal
			}
			used, _ := h.predictions.CountDoubleDowns(ctx, userID, matchID)
			st.ddRemaining = model.DoubleDownLimit - used
		}
	}
	if err := c.Respond(); err != nil {
		return err
	}
	var edited *tele.Message
	if hasPrediction {
		edited, err = c.Bot().Edit(c.Message(), buildPredictMsg(st), buildPredictKeyboard(st), tele.ModeMarkdown)
	} else {
		edited, err = c.Bot().Edit(c.Message(), msg, tele.ModeMarkdown)
	}
	if err != nil {
		return err
	}
	st.msgID = edited.ID
	h.store.set(c.Sender().ID, st)
	return nil
}

func groupStandingsBlock(ctx context.Context, h *Handler, m *model.Match) string {
	if m.Group == nil {
		return ""
	}
	entries, err := h.matches.GetGroupStandings(ctx, *m.Group)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return formatGroupStandings(*m.Group, entries)
}

func formatExistingBet(standingsBlock string, m *model.Match, p *model.Prediction, canEdit bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%s — %s*\n\n", withFlag(m.HomeTeam), withFlag(m.AwayTeam)))
	if standingsBlock != "" {
		sb.WriteString(standingsBlock)
		sb.WriteString("\n")
	}
	sb.WriteString("Твоя ставка: " + predSummary(p, m.HomeTeam, m.AwayTeam))
	if p.DoubleDown {
		sb.WriteString(" 🔥")
	}
	if extras := specialExtras(p); extras != "" {
		sb.WriteString(" " + extras)
	}
	sb.WriteString("\n")
	if !canEdit {
		sb.WriteString("\n_Ставки закрыты_")
	}
	return sb.String()
}

func buildExistingBetKeyboard(matchID int64, canEdit bool) *tele.ReplyMarkup {
	rows := [][]tele.InlineButton{}
	if canEdit {
		rows = append(rows, []tele.InlineButton{
			{Text: "Изменить ставку", Data: fmt.Sprintf("eb|%d", matchID)},
		})
	}
	rows = append(rows, []tele.InlineButton{
		{Text: "← Матчи", Data: "back|m"},
	})
	return &tele.ReplyMarkup{InlineKeyboard: rows}
}

func backToMatchesKeyboard() *tele.ReplyMarkup {
	return &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{{Text: "← Матчи", Data: "back|m"}},
		},
	}
}


func buildMatchBetMsg(ctx context.Context, h *Handler, m *model.Match) string {
	header := fmt.Sprintf("*%s — %s*\n\n", withFlag(m.HomeTeam), withFlag(m.AwayTeam))
	prompt := "Введи точный счёт\nПример: 2:1"
	block := groupStandingsBlock(ctx, h, m)
	if block == "" {
		return header + prompt
	}
	return header + block + "\n" + prompt
}

func formatGroupStandings(groupName string, entries []model.StandingEntry) string {
	// Strip "GROUP_" prefix: "GROUP_A" → "A"
	label := strings.TrimPrefix(groupName, "GROUP_")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Группа %s*\n", label))
	sb.WriteString("```\n")
	sb.WriteString(" #  Команда              И  В  Н  П  О\n")
	for i, e := range entries {
		name := e.Team
		if len(name) > 20 {
			name = name[:19] + "."
		}
		sb.WriteString(fmt.Sprintf(" %d  %-20s %d  %d  %d  %d  %d\n",
			i+1, name, e.Played, e.Won, e.Drawn, e.Lost, e.Points))
	}
	sb.WriteString("```\n\n")
	return sb.String()
}


func (h *Handler) parseExactScore(c tele.Context, st *predictionState, text string) error {
	parts := strings.Split(text, ":")
	if len(parts) != 2 {
		return h.editPrompt(c, st, fmt.Sprintf("*%s — %s*\n\nВведи точный счёт\nПример: 2:1\n\n⚠️ Неверный формат", withFlag(st.homeTeam), withFlag(st.awayTeam)))
	}
	home, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	away, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || home < 0 || away < 0 {
		return h.editPrompt(c, st, fmt.Sprintf("*%s — %s*\n\nВведи точный счёт\nПример: 2:1\n\n⚠️ Неверный счёт", withFlag(st.homeTeam), withFlag(st.awayTeam)))
	}
	st.homeScore = home
	st.awayScore = away
	return h.showPredictForm(c, st)
}


func (h *Handler) editPrompt(c tele.Context, st *predictionState, text string) error {
	editable := &tele.Message{ID: st.msgID, Chat: c.Chat()}
	_, err := c.Bot().Edit(editable, text, tele.ModeMarkdown)
	return err
}


func (h *Handler) showPredictForm(c tele.Context, st *predictionState) error {
	ctx := context.Background()
	userID, err := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if err == nil {
		used, _ := h.predictions.CountDoubleDowns(ctx, userID, st.matchID)
		st.ddRemaining = model.DoubleDownLimit - used
	}

	editable := &tele.Message{ID: st.msgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(editable, buildPredictMsg(st), buildPredictKeyboard(st), tele.ModeMarkdown)
	return err
}


func (h *Handler) handleSpecialToggle(c tele.Context, bet string) error {
	st, ok := h.store.get(c.Sender().ID)
	if !ok {
		return c.Respond()
	}
	next := specialBet(bet)
	if st.special == next {
		st.special = specialNone
	} else {
		st.special = next
	}
	return h.editPredictMsg(c, st)
}

func (h *Handler) handleDoubleDownToggle(c tele.Context) error {
	st, ok := h.store.get(c.Sender().ID)
	if !ok {
		return c.Respond()
	}
	if !st.doubleDown {
		ctx := context.Background()
		userID, err := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
		if err != nil {
			return err
		}
		used, err := h.predictions.CountDoubleDowns(ctx, userID, st.matchID)
		if err != nil {
			return err
		}
		if used >= model.DoubleDownLimit {
			return c.Respond(&tele.CallbackResponse{
				Text: fmt.Sprintf("Лимит Double Down исчерпан (%d из %d)", used, model.DoubleDownLimit),
			})
		}
	}
	st.doubleDown = !st.doubleDown
	return h.editPredictMsg(c, st)
}

func (h *Handler) handleSave(c tele.Context) error {
	st, ok := h.store.get(c.Sender().ID)
	if !ok {
		return c.Respond()
	}

	ctx := context.Background()

	m, err := h.matches.GetByID(ctx, st.matchID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Матч не найден"})
	}
	if m.Status != model.MatchStatusTimed || time.Until(m.MatchDate) <= 5*time.Minute {
		h.store.del(c.Sender().ID)
		return c.Respond(&tele.CallbackResponse{Text: "Ставки на этот матч уже закрыты"})
	}

	userID, err := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		return err
	}

	p := &model.Prediction{
		UserID:     userID,
		MatchID:    st.matchID,
		BetType:    st.betType,
		HomeScore:  st.homeScore,
		AwayScore:  st.awayScore,
		DoubleDown: st.doubleDown,
		BetPenalty: st.special == specialPenalty,
		BetRedCard: st.special == specialRedCard,
		BetOwnGoal: st.special == specialOwnGoal,
	}

	if err := h.predictions.Upsert(ctx, p); err != nil {
		return err
	}

	// Auto-register user in the group they came from (encoded in the deeplink).
	if st.groupChatID != 0 {
		if g, err := h.groups.GetByChatID(ctx, st.groupChatID); err == nil {
			h.groups.AddMember(ctx, g.ID, userID) //nolint:errcheck
		}
	}

	h.store.del(c.Sender().ID)
	if err := c.Respond(&tele.CallbackResponse{Text: "Ставка принята!"}); err != nil {
		return err
	}

	_, err = c.Bot().Edit(
		c.Message(),
		fmt.Sprintf("Ставка сохранена: *%s*%s%s",
			betSummary(st),
			specialLabel(st.special),
			ddLabel(st.doubleDown),
		),
		backToMatchesKeyboard(),
		tele.ModeMarkdown,
	)
	return err
}

func (h *Handler) editPredictMsg(c tele.Context, st *predictionState) error {
	if err := c.Respond(); err != nil {
		return err
	}
	_, err := c.Bot().Edit(c.Message(), buildPredictMsg(st), buildPredictKeyboard(st), tele.ModeMarkdown)
	if isNotModified(err) {
		return nil
	}
	return err
}

func buildPredictMsg(st *predictionState) string {
	ddLine := fmt.Sprintf("Double Down осталось: %d из %d", st.ddRemaining, model.DoubleDownLimit)
	if st.ddRemaining <= 0 {
		ddLine = "Double Down исчерпан"
	}
	return fmt.Sprintf(
		"*%s — %s*\nПрогноз: %s\n%s",
		withFlag(st.homeTeam), withFlag(st.awayTeam), betSummary(st), ddLine,
	)
}

func betSummary(st *predictionState) string {
	return fmt.Sprintf("%d:%d", st.homeScore, st.awayScore)
}


func buildPredictKeyboard(st *predictionState) *tele.ReplyMarkup {
	check := func(s specialBet) string {
		if st.special == s {
			return "✓ "
		}
		return ""
	}

	ddText := "🔥Double Down x2"
	if st.doubleDown {
		ddText = "✓ Double Down x2"
	}

	rows := [][]tele.InlineButton{}

	// DD — отдельная строка, выше спец ставок
	if st.ddRemaining > 0 || st.doubleDown {
		rows = append(rows, []tele.InlineButton{
			{Text: ddText, Data: "dd"},
		})
	}

	rows = append(rows,
		[]tele.InlineButton{{Text: check(specialPenalty) + "🥅 Пенальти +2/−1", Data: "s|penalty"}},
		[]tele.InlineButton{{Text: check(specialRedCard) + "🟥 Красная +3/−2", Data: "s|red_card"}},
		[]tele.InlineButton{{Text: check(specialOwnGoal) + "🤦 Автогол +5/−3", Data: "s|own_goal"}},
		[]tele.InlineButton{{Text: "Сохранить", Data: "sv"}, {Text: "← Матчи", Data: "back|m"}},
	)

	return &tele.ReplyMarkup{InlineKeyboard: rows}
}

func specialLabel(s specialBet) string {
	switch s {
	case specialPenalty:
		return " · Пенальти"
	case specialRedCard:
		return " · Красная"
	case specialOwnGoal:
		return " · Автогол"
	default:
		return ""
	}
}

func ddLabel(dd bool) string {
	if dd {
		return " · Double Down"
	}
	return ""
}
