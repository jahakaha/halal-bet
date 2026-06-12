package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

func (h *Handler) Bets(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return c.Send("Ставки доступны только в групповом чате.")
	}

	_ = h.registerGroupMember(c)

	ctx := context.Background()

	group, err := h.groups.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		return c.Send("Группа не зарегистрирована. Напишите /start сначала.")
	}

	from, to := betsWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return c.Send("Матчей нет.")
	}

	var sb strings.Builder
	for _, m := range matches {
		sb.WriteString(formatMatchBets(ctx, h, m, group.ID))
		sb.WriteString("\n")
	}

	return c.Send(sb.String(), tele.ModeHTML)
}

func formatMatchBets(ctx context.Context, h *Handler, m model.Match, groupID int64) string {
	var sb strings.Builder

	localTime := m.MatchDate.In(almatyLoc).Format("15:04")
	hasStarted := m.Status == model.MatchStatusInPlay ||
		m.Status == model.MatchStatusPaused ||
		m.Status == model.MatchStatusFinished ||
		m.MatchDate.Before(time.Now())

	switch m.Status {
	case model.MatchStatusFinished:
		score := ""
		if m.HomeScore != nil && m.AwayScore != nil {
			score = fmt.Sprintf(" %d:%d", *m.HomeScore, *m.AwayScore)
		}
		sb.WriteString(fmt.Sprintf("✅ <b>%s — %s</b>%s\n", m.HomeTeam, m.AwayTeam, score))
	case model.MatchStatusInPlay:
		sb.WriteString(fmt.Sprintf("🟢 <b>%s — %s</b> (идёт)\n", m.HomeTeam, m.AwayTeam))
	case model.MatchStatusPaused:
		sb.WriteString(fmt.Sprintf("⏸ <b>%s — %s</b> (перерыв)\n", m.HomeTeam, m.AwayTeam))
	default:
		sb.WriteString(fmt.Sprintf("🕐 <b>%s — %s</b>  %s\n", m.HomeTeam, m.AwayTeam, localTime))
	}

	if !hasStarted {
		sb.WriteString("  <i>матч ещё не стартовал</i>\n")
		return sb.String()
	}

	var preds []model.PredictionWithUser
	var err error
	if groupID != 0 {
		preds, err = h.predictions.GetByMatchWithUsersInGroup(ctx, m.ID, groupID)
	} else {
		preds, err = h.predictions.GetByMatchWithUsers(ctx, m.ID)
	}
	if err != nil || len(preds) == 0 {
		sb.WriteString("  <i>нет ставок</i>\n")
		return sb.String()
	}

	for _, p := range preds {
		name := p.Username
		if name == "" {
			name = "Аноним"
		}
		line := fmt.Sprintf("  %s: %s", name, predSummary(&p.Prediction, m.HomeTeam, m.AwayTeam))
		if p.DoubleDown {
			line += " 🔥"
		}
		if extras := specialExtras(&p.Prediction); extras != "" {
			line += " " + extras
		}
		if p.Points != nil {
			line += fmt.Sprintf(" → <b>%+d</b>", *p.Points)
		}
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

func predSummary(p *model.Prediction, _, _ string) string {
	return fmt.Sprintf("%d:%d", p.HomeScore, p.AwayScore)
}

func specialExtras(p *model.Prediction) string {
	var parts []string
	if p.BetPenalty {
		parts = append(parts, "🥅")
	}
	if p.BetRedCard {
		parts = append(parts, "🟥")
	}
	if p.BetOwnGoal {
		parts = append(parts, "🤦")
	}
	return strings.Join(parts, "")
}

