package bot

import (
	"context"
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

func (h *Handler) Stats(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return h.sendPrivateOnlyHint(c)
	}

	ctx := context.Background()
	group, err := h.groups.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		return c.Send("Группа не зарегистрирована. Напишите /start сначала.")
	}

	stats, err := h.groups.GetGroupStats(ctx, group.ID)
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		return c.Send("Пока нет данных.")
	}

	return c.Send(buildStatsMsg(stats), tele.ModeHTML)
}

func buildStatsMsg(stats []model.UserStats) string {
	var sb strings.Builder
	sb.WriteString("<b>📊 Статистика группы</b>\n\n")

	for _, s := range stats {
		name := s.Username
		if name == "" {
			name = "Аноним"
		}
		sb.WriteString(fmt.Sprintf("<b>@%s</b> — %d ставок\n", name, s.Total))
		sb.WriteString(fmt.Sprintf(
			"🎯 %d (%d%%)  ↕️ %d (%d%%)  ✓ %d (%d%%)  ✗ %d (%d%%)\n",
			s.Exact, calcPct(s.Exact, s.Total),
			s.Diff, calcPct(s.Diff, s.Total),
			s.Outcome, calcPct(s.Outcome, s.Total),
			s.Miss(), calcPct(s.Miss(), s.Total),
		))

		var risky []string
		if s.PenaltyBets > 0 {
			risky = append(risky, fmt.Sprintf("🥅 %d/%d", s.PenaltyHits, s.PenaltyBets))
		}
		if s.RedCardBets > 0 {
			risky = append(risky, fmt.Sprintf("🟥 %d/%d", s.RedCardHits, s.RedCardBets))
		}
		if s.OwnGoalBets > 0 {
			risky = append(risky, fmt.Sprintf("🤦 %d/%d", s.OwnGoalHits, s.OwnGoalBets))
		}
		if len(risky) > 0 {
			sb.WriteString(strings.Join(risky, "  ") + "\n")
		}
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func calcPct(n, total int) int {
	if total == 0 {
		return 0
	}
	return n * 100 / total
}
