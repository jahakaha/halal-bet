package bot

import (
	"context"
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

var wc2026Groups = []string{
	"GROUP_A", "GROUP_B", "GROUP_C", "GROUP_D",
	"GROUP_E", "GROUP_F", "GROUP_G", "GROUP_H",
	"GROUP_I", "GROUP_J", "GROUP_K", "GROUP_L",
}

// Groups shows a grid of group buttons (A–L).
func (h *Handler) Groups(c tele.Context) error {
	if c.Chat().Type != tele.ChatPrivate {
		return h.sendPrivateOnlyHint(c)
	}

	rows := make([][]tele.InlineButton, 0, 3)
	for i := 0; i < len(wc2026Groups); i += 4 {
		var row []tele.InlineButton
		for j := i; j < i+4 && j < len(wc2026Groups); j++ {
			g := wc2026Groups[j]
			row = append(row, tele.InlineButton{
				Text: "Группа " + g[len("GROUP_"):],
				Data: "grp|" + g,
			})
		}
		rows = append(rows, row)
	}
	return c.Send("Выбери группу:", &tele.ReplyMarkup{InlineKeyboard: rows})
}

// handleGroupStandings is called from OnCallback when Data has "grp|" prefix.
func (h *Handler) handleGroupStandings(c tele.Context, groupName string) error {
	entries, err := h.matches.GetGroupStandings(context.Background(), groupName)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return c.Respond(&tele.CallbackResponse{Text: "Данных пока нет"})
	}
	_ = c.Respond()
	return c.Send(formatGroupTable(groupName, entries), tele.ModeMarkdown)
}

func formatGroupTable(groupName string, entries []model.StandingEntry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 *%s*\n", groupLabel(groupName)))
	sb.WriteString("```\n")
	sb.WriteString(" #  Команда            И  В  Н  П   О\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("%2d  %-18s %d  %d  %d  %d  %2d\n",
			i+1, e.Team, e.Played, e.Won, e.Drawn, e.Lost, e.Points))
	}
	sb.WriteString("```")
	return sb.String()
}

func groupLabel(name string) string {
	if strings.HasPrefix(strings.ToUpper(name), "GROUP_") {
		return "Группа " + name[len("GROUP_"):]
	}
	return strings.ReplaceAll(name, "_", " ")
}
