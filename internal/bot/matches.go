package bot

import (
	"context"
	"fmt"
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

func tomorrowWindow() (from, to time.Time) {
	now := time.Now().In(almatyLoc)
	if testDate != nil {
		now = testDate.In(almatyLoc)
	}
	from = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, almatyLoc).UTC()
	to = from.Add(24 * time.Hour)
	return
}

// todayWindow returns today's 24h range in Almaty time.
// SetTestDate overrides today's date for testing. Pass nil to reset.
var testDate *time.Time

func SetTestDate(d *time.Time) { testDate = d }

func todayWindow() (from, to time.Time) {
	now := time.Now().In(almatyLoc)
	if testDate != nil {
		now = testDate.In(almatyLoc)
	}
	from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, almatyLoc).UTC()
	to = from.Add(24 * time.Hour)
	return
}

func (h *Handler) Matches(c tele.Context) error {
	if c.Chat().Type != tele.ChatPrivate {
		return h.sendPrivateOnlyHint(c)
	}

	ctx := context.Background()
	from, to := tomorrowWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return err
	}

	var upcoming []model.Match
	for _, m := range matches {
		if m.Status == model.MatchStatusTimed {
			upcoming = append(upcoming, m)
		}
	}

	if len(upcoming) == 0 {
		return c.Send("Матчей завтра нет.")
	}

	seen := make(map[int64]bool)
	rows := make([][]tele.InlineButton, 0, len(upcoming))
	for _, m := range upcoming {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		localTime := m.MatchDate.In(almatyLoc).Format("15:04")
		label := fmt.Sprintf("%s — %s  %s", m.HomeTeam, m.AwayTeam, localTime)
		btn := tele.InlineButton{
			Text: label,
			Data: fmt.Sprintf("m|%d", m.ID),
		}
		rows = append(rows, []tele.InlineButton{btn})
	}

	return c.Send("Матчи сегодня:", &tele.ReplyMarkup{InlineKeyboard: rows})
}

func (h *Handler) handleBackToMatches(c tele.Context) error {
	ctx := context.Background()
	from, to := tomorrowWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return c.Respond()
	}

	seen := make(map[int64]bool)
	rows := make([][]tele.InlineButton, 0, len(matches))
	for _, m := range matches {
		if m.Status != model.MatchStatusTimed || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		localTime := m.MatchDate.In(almatyLoc).Format("15:04")
		label := fmt.Sprintf("%s — %s  %s", m.HomeTeam, m.AwayTeam, localTime)
		rows = append(rows, []tele.InlineButton{{
			Text: label,
			Data: fmt.Sprintf("m|%d", m.ID),
		}})
	}

	if err := c.Respond(); err != nil {
		return err
	}

	if len(rows) == 0 {
		_, err := c.Bot().Edit(c.Message(), "Матчей завтра нет.")
		return err
	}

	_, err = c.Bot().Edit(c.Message(), "Матчи сегодня:", &tele.ReplyMarkup{InlineKeyboard: rows})
	return err
}
