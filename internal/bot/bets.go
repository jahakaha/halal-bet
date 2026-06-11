package bot

import (
	"context"
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

func (h *Handler) Bets(c tele.Context) error {
	_ = h.registerGroupMember(c)

	ctx := context.Background()

	var groupID int64
	if c.Chat().Type != tele.ChatPrivate {
		if g, err := h.groups.GetByChatID(ctx, c.Chat().ID); err == nil {
			groupID = g.ID
		}
	}

	from, to := todayWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return err
	}

	var started []model.Match
	for _, m := range matches {
		if m.Status == model.MatchStatusInPlay || m.Status == model.MatchStatusPaused || m.Status == model.MatchStatusFinished {
			started = append(started, m)
		}
	}

	if len(started) == 0 {
		return c.Send("Матчи ещё не начались.")
	}

	var sb strings.Builder
	for _, m := range started {
		sb.WriteString(formatMatchBets(ctx, h, m, groupID))
		sb.WriteString("\n")
	}

	return c.Send(sb.String(), tele.ModeMarkdown)
}

func formatMatchBets(ctx context.Context, h *Handler, m model.Match, groupID int64) string {
	var sb strings.Builder

	switch m.Status {
	case model.MatchStatusFinished:
		score := ""
		if m.HomeScore != nil && m.AwayScore != nil {
			score = fmt.Sprintf(" %d:%d", *m.HomeScore, *m.AwayScore)
		}
		sb.WriteString(fmt.Sprintf("✅ *%s — %s*%s\n", m.HomeTeam, m.AwayTeam, score))
	case model.MatchStatusInPlay:
		sb.WriteString(fmt.Sprintf("🟢 *%s — %s* (идёт)\n", m.HomeTeam, m.AwayTeam))
	case model.MatchStatusPaused:
		sb.WriteString(fmt.Sprintf("⏸ *%s — %s* (перерыв)\n", m.HomeTeam, m.AwayTeam))
	}

	var preds []model.PredictionWithUser
	var err error
	if groupID != 0 {
		preds, err = h.predictions.GetByMatchWithUsersInGroup(ctx, m.ID, groupID)
	} else {
		preds, err = h.predictions.GetByMatchWithUsers(ctx, m.ID)
	}
	if err != nil || len(preds) == 0 {
		sb.WriteString("  _нет ставок_\n")
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
			line += fmt.Sprintf(" → *%+d*", *p.Points)
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

