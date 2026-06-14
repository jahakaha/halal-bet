package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

func (h *Handler) Me(c tele.Context) error {
	if c.Chat().Type != tele.ChatPrivate {
		return h.sendPrivateOnlyHint(c)
	}
	return h.sendMePage(c, "", false)
}

func (h *Handler) handleMeNav(c tele.Context) error {
	// callback data: "me|YYYY-MM-DD"
	date := strings.TrimPrefix(c.Data(), "me|")
	if err := c.Respond(); err != nil {
		return err
	}
	return h.sendMePage(c, date, true)
}

func (h *Handler) sendMePage(c tele.Context, dateKey string, edit bool) error {
	ctx := context.Background()

	userID, err := h.users.GetIDByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		return c.Send("Сначала напиши /start.")
	}

	history, err := h.predictions.GetHistoryByUser(ctx, userID)
	if err != nil {
		return err
	}
	if len(history) == 0 {
		return c.Send("У тебя ещё нет ставок.")
	}

	// Group by Almaty calendar date.
	type dayGroup struct {
		key   string
		date  time.Time
		items []model.PredictionWithMatch
	}
	var days []dayGroup
	dayIdx := map[string]int{}

	for _, pw := range history {
		key := pw.MatchDate.In(almatyLoc).Format("2006-01-02")
		idx, ok := dayIdx[key]
		if !ok {
			idx = len(days)
			days = append(days, dayGroup{key: key, date: pw.MatchDate})
			dayIdx[key] = idx
		}
		days[idx].items = append(days[idx].items, pw)
	}

	// Default: most recent day that has started.
	if dateKey == "" {
		now := time.Now().In(almatyLoc)
		for i := len(days) - 1; i >= 0; i-- {
			if days[i].date.Before(now) {
				dateKey = days[i].key
				break
			}
		}
		if dateKey == "" {
			dateKey = days[0].key
		}
	}

	curIdx, ok := dayIdx[dateKey]
	if !ok {
		curIdx = len(days) - 1
	}
	cur := days[curIdx]

	// Build message.
	msg := buildMeMsg(cur.date, cur.items)

	// Build nav keyboard.
	var navRow []tele.InlineButton
	if curIdx > 0 {
		prev := days[curIdx-1]
		navRow = append(navRow, tele.InlineButton{
			Text: "← " + matchDateLabel(prev.date),
			Data: "me|" + prev.key,
		})
	}
	if curIdx < len(days)-1 {
		next := days[curIdx+1]
		navRow = append(navRow, tele.InlineButton{
			Text: matchDateLabel(next.date) + " →",
			Data: "me|" + next.key,
		})
	}

	var markup *tele.ReplyMarkup
	if len(navRow) > 0 {
		markup = &tele.ReplyMarkup{InlineKeyboard: [][]tele.InlineButton{navRow}}
	}

	if edit {
		_, err = c.Bot().Edit(c.Message(), msg, markup, tele.ModeHTML)
		if isNotModified(err) {
			return nil
		}
		return err
	}
	return c.Send(msg, markup, tele.ModeHTML)
}

func buildMeMsg(date time.Time, items []model.PredictionWithMatch) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>📊 Мои ставки — %s</b>\n\n", matchDateLabel(date)))

	totalPts := 0
	for _, pw := range items {
		m := matchFromPW(&pw)
		sb.WriteString(meMatchLine(&pw, m))
		if pw.Points != nil {
			totalPts += *pw.Points
		}
	}

	// Summary line.
	exact, outcome, wrong := 0, 0, 0
	for _, pw := range items {
		if pw.ActualHomeScore == nil {
			continue
		}
		base := baseResultLabel(pw.Prediction, *pw.ActualHomeScore, *pw.ActualAwayScore)
		switch base {
		case "🎯", "↕️":
			exact++
		case "✓":
			outcome++
		default:
			wrong++
		}
	}

	finished := exact + outcome + wrong
	if finished > 0 {
		sb.WriteString(fmt.Sprintf("\n<b>Итого: %+d</b>  |  🎯%d  ✓%d  ✗%d", totalPts, exact, outcome, wrong))
	}
	return sb.String()
}

func meMatchLine(pw *model.PredictionWithMatch, m *model.Match) string {
	var sb strings.Builder

	// Match header.
	localTime := pw.MatchDate.In(almatyLoc).Format("15:04")
	switch pw.MatchStatus {
	case model.MatchStatusFinished:
		score := ""
		if pw.ActualHomeScore != nil && pw.ActualAwayScore != nil {
			score = fmt.Sprintf(" %d:%d", *pw.ActualHomeScore, *pw.ActualAwayScore)
		}
		sb.WriteString(fmt.Sprintf("✅ <b>%s — %s</b>%s\n", pw.HomeTeam, pw.AwayTeam, score))
	case model.MatchStatusInPlay:
		sb.WriteString(fmt.Sprintf("🟢 <b>%s — %s</b>\n", pw.HomeTeam, pw.AwayTeam))
	case model.MatchStatusPaused:
		sb.WriteString(fmt.Sprintf("⏸ <b>%s — %s</b>\n", pw.HomeTeam, pw.AwayTeam))
	default:
		sb.WriteString(fmt.Sprintf("🕐 <b>%s — %s</b>  %s\n", pw.HomeTeam, pw.AwayTeam, localTime))
	}

	// Prediction line.
	parts := []string{fmt.Sprintf("   %d:%d", pw.Prediction.HomeScore, pw.Prediction.AwayScore)}

	if pw.ActualHomeScore != nil {
		parts = append(parts, baseResultLabel(pw.Prediction, *pw.ActualHomeScore, *pw.ActualAwayScore))
	}
	if pw.DoubleDown {
		if pw.ActualHomeScore != nil && model.CalcPoints(&pw.Prediction, m) != nil {
			base := model.OutcomeOf(pw.Prediction.HomeScore, pw.Prediction.AwayScore)
			actual := model.OutcomeOf(*pw.ActualHomeScore, *pw.ActualAwayScore)
			if base == actual {
				parts = append(parts, "🔥×2")
			} else {
				parts = append(parts, "💀DD")
			}
		} else {
			parts = append(parts, "🔥")
		}
	}

	// Risky bets.
	if pw.BetPenalty {
		parts = append(parts, riskyLabel("🥅", pw.HadPenalty))
	}
	if pw.BetRedCard {
		parts = append(parts, riskyLabel("🟥", pw.HadRedCard))
	}
	if pw.BetOwnGoal {
		parts = append(parts, riskyLabel("🤦", pw.HadOwnGoal))
	}

	// Points.
	if pts := model.CalcPoints(&pw.Prediction, m); pts != nil {
		parts = append(parts, fmt.Sprintf("→ <b>%+d</b>", *pts))
	} else if pw.ActualHomeScore == nil {
		parts = append(parts, "→ ?")
	}

	sb.WriteString(strings.Join(parts, "  "))
	sb.WriteString("\n")
	return sb.String()
}

func baseResultLabel(p model.Prediction, actualHome, actualAway int) string {
	if p.HomeScore == actualHome && p.AwayScore == actualAway {
		return "🎯"
	}
	if p.HomeScore-p.AwayScore == actualHome-actualAway {
		return "↕️"
	}
	if model.OutcomeOf(p.HomeScore, p.AwayScore) == model.OutcomeOf(actualHome, actualAway) {
		return "✓"
	}
	return "✗"
}

func riskyLabel(icon string, had *bool) string {
	if had == nil {
		return icon + "?"
	}
	if *had {
		return icon + "✓"
	}
	return icon + "✗"
}

// matchFromPW builds a model.Match from PredictionWithMatch for CalcPoints.
func matchFromPW(pw *model.PredictionWithMatch) *model.Match {
	return &model.Match{
		ID:         pw.MatchID,
		HomeTeam:   pw.HomeTeam,
		AwayTeam:   pw.AwayTeam,
		MatchDate:  pw.MatchDate,
		Status:     pw.MatchStatus,
		HomeScore:  pw.ActualHomeScore,
		AwayScore:  pw.ActualAwayScore,
		HadPenalty: pw.HadPenalty,
		HadRedCard: pw.HadRedCard,
		HadOwnGoal: pw.HadOwnGoal,
	}
}
