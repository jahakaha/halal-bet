package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
)

var almatyLoc = mustLoadLocation("Asia/Almaty")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// matchWindow returns the range for /matches — extended to 72h during UCL final test.
// TODO: revert to 24h after test.
func matchWindow() (from, to time.Time) {
	from, _ = todayWindow()
	to = from.Add(72 * time.Hour)
	return
}

// todayWindow returns today's 24h range in Almaty time.
func todayWindow() (from, to time.Time) {
	now := time.Now().In(almatyLoc)
	from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, almatyLoc).UTC()
	to = from.Add(24 * time.Hour)
	return
}

func (h *Handler) Matches(c tele.Context) error {
	ctx := context.Background()
	from, to := matchWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return c.Send("Сегодня матчей нет.")
	}

	botUsername := c.Bot().Me.Username
	rows := make([][]tele.InlineButton, 0, len(matches))
	for _, m := range matches {
		localTime := m.MatchDate.In(almatyLoc).Format("15:04")
		label := fmt.Sprintf("%s — %s  %s", m.HomeTeam, m.AwayTeam, localTime)
		btn := tele.InlineButton{
			Text: label,
			URL:  fmt.Sprintf("https://t.me/%s?start=m_%d", botUsername, m.ID),
		}
		rows = append(rows, []tele.InlineButton{btn})
	}

	return c.Send("Матчи сегодня:", &tele.ReplyMarkup{InlineKeyboard: rows})
}

func matchCaption(m model.Match) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%s — %s*\n", m.HomeTeam, m.AwayTeam))

	localTime := m.MatchDate.In(almatyLoc).Format("02 Jan · 15:04")
	if m.Group != nil {
		sb.WriteString(fmt.Sprintf("%s · %s алм.\n", strings.ReplaceAll(*m.Group, "_", " "), localTime))
	} else {
		sb.WriteString(fmt.Sprintf("%s алм.\n", localTime))
	}

	sb.WriteString("\nВведи счёт (например: 2:1)")
	return sb.String()
}
