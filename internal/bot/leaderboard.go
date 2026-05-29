package bot

import (
	"context"
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

func (h *Handler) Leaderboard(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return c.Send("Таблица доступна только в групповом чате.")
	}

	ctx := context.Background()
	group, err := h.groups.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		return c.Send("Группа не зарегистрирована. Напишите /start сначала.")
	}

	entries, isLive, err := h.leaderboard.LiveLeaderboard(ctx, group.ID)
	if err != nil {
		return err
	}

	return c.Send(buildLeaderboardMsg(entries, isLive), tele.ModeMarkdown)
}

func buildLeaderboardMsg(entries []model.GroupLeaderboardEntry, isLive bool) string {
	if len(entries) == 0 {
		return "Ещё никто не делал ставок."
	}

	var sb strings.Builder
	title := "🏆 *Таблица*"
	if isLive {
		title += " 🟢"
	}
	sb.WriteString(title + "\n\n")

	medals := []string{"🥇", "🥈", "🥉"}

	for i, e := range entries {
		var rank string
		if i < len(medals) {
			rank = medals[i]
		} else {
			rank = fmt.Sprintf("%d.", i+1)
		}

		name := e.Username
		if name == "" {
			name = "Аноним"
		}

		total := e.Total()
		line := fmt.Sprintf("%s *%s* — %d %s", rank, name, total, pointWord(total))
		if e.LivePoints != 0 {
			line += fmt.Sprintf(" (%+d в игре)", e.LivePoints)
		}
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

func pointWord(n int) string {
	switch {
	case n < 0:
		return "очков"
	case n == 1:
		return "очко"
	case n >= 2 && n <= 4:
		return "очка"
	default:
		return "очков"
	}
}

func betWord(n int) string {
	switch {
	case n == 1:
		return "ставка"
	case n >= 2 && n <= 4:
		return "ставки"
	default:
		return "ставок"
	}
}
