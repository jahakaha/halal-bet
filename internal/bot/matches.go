package bot

import (
	"context"
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"
)

var almatyLoc = mustLoadLocation("Asia/Almaty")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// matchWindow returns matches for today and tomorrow (48h from midnight).
func matchWindow() (from, to time.Time) {
	from, _ = todayWindow()
	to = from.Add(48 * time.Hour)
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

