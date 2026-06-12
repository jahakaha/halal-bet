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

// matchesWindow returns today's matches before 12:00 Almaty, tomorrow's after 12:00.
func matchesWindow() (from, to time.Time) {
	now := time.Now().In(almatyLoc)
	if testDate != nil {
		now = testDate.In(almatyLoc)
	}
	offset := 1
	if now.Hour() < 12 {
		offset = 0
	}
	from = time.Date(now.Year(), now.Month(), now.Day()+offset, 0, 0, 0, 0, almatyLoc).UTC()
	to = from.Add(24 * time.Hour)
	return
}

// SetTestDate overrides today's date for testing. Pass nil to reset.
var testDate *time.Time

func SetTestDate(d *time.Time) { testDate = d }

// betsWindow returns yesterday before 18:00 Almaty, today after 18:00.
// Before 18:00 shows yesterday's results; after 18:00 switches to today
// so bets are hidden until matches start (building anticipation).
func betsWindow() (from, to time.Time) {
	now := time.Now().In(almatyLoc)
	if testDate != nil {
		now = testDate.In(almatyLoc)
	}
	offset := -1
	if now.Hour() >= 18 {
		offset = 0
	}
	from = time.Date(now.Year(), now.Month(), now.Day()+offset, 0, 0, 0, 0, almatyLoc).UTC()
	to = from.Add(24 * time.Hour)
	return
}

func matchesLabel() string {
	now := time.Now().In(almatyLoc)
	if now.Hour() < 12 {
		return "Матчи сегодня:"
	}
	return "Матчи завтра:"
}

func (h *Handler) Matches(c tele.Context) error {
	if c.Chat().Type != tele.ChatPrivate {
		return h.sendPrivateOnlyHint(c)
	}

	ctx := context.Background()
	from, to := matchesWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return c.Send("Матчей нет.")
	}

	seen := make(map[int64]bool)
	rows := make([][]tele.InlineButton, 0, len(matches))
	for _, m := range matches {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		localTime := m.MatchDate.In(almatyLoc).Format("15:04")
		prefix := ""
		switch m.Status {
		case model.MatchStatusInPlay:
			prefix = "🟢 "
		case model.MatchStatusPaused:
			prefix = "⏸ "
		case model.MatchStatusFinished:
			prefix = "✅ "
		}
		label := fmt.Sprintf("%s%s — %s  %s", prefix, m.HomeTeam, m.AwayTeam, localTime)
		btn := tele.InlineButton{
			Text: label,
			Data: fmt.Sprintf("m|%d", m.ID),
		}
		rows = append(rows, []tele.InlineButton{btn})
	}

	return c.Send(matchesLabel(), &tele.ReplyMarkup{InlineKeyboard: rows})
}

func (h *Handler) handleBackToMatches(c tele.Context) error {
	ctx := context.Background()
	from, to := matchesWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return c.Respond()
	}

	seen := make(map[int64]bool)
	rows := make([][]tele.InlineButton, 0, len(matches))
	for _, m := range matches {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		localTime := m.MatchDate.In(almatyLoc).Format("15:04")
		prefix := ""
		switch m.Status {
		case model.MatchStatusInPlay:
			prefix = "🟢 "
		case model.MatchStatusPaused:
			prefix = "⏸ "
		case model.MatchStatusFinished:
			prefix = "✅ "
		}
		label := fmt.Sprintf("%s%s — %s  %s", prefix, m.HomeTeam, m.AwayTeam, localTime)
		rows = append(rows, []tele.InlineButton{{
			Text: label,
			Data: fmt.Sprintf("m|%d", m.ID),
		}})
	}

	if err := c.Respond(); err != nil {
		return err
	}

	if len(rows) == 0 {
		_, err := c.Bot().Edit(c.Message(), "Матчей нет.")
		return err
	}

	_, err = c.Bot().Edit(c.Message(), matchesLabel(), &tele.ReplyMarkup{InlineKeyboard: rows})
	return err
}
