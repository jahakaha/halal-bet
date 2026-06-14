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

// matchesWindow returns the window for /matches.
// Before 12:00: today's calendar day (matches already running, e.g. 03:00, 07:00).
// After 12:00: current game night window (today noon → tomorrow noon),
// which covers tonight (22:00+) and tomorrow morning (00:00–11:00) together.
func matchesWindow() (from, to time.Time) {
	now := time.Now().In(almatyLoc)
	if testDate != nil {
		now = testDate.In(almatyLoc)
	}
	if now.Hour() < 12 {
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, almatyLoc).UTC()
	} else {
		from = time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, almatyLoc).UTC()
	}
	to = from.Add(24 * time.Hour)
	return
}

// SetTestDate overrides today's date for testing. Pass nil to reset.
var testDate *time.Time

func SetTestDate(d *time.Time) { testDate = d }

// betsWindow returns the game-night window for /bets.
// WC2026 matches run 22:00–11:00 Almaty. The window is noon-to-noon.
// Before 18:00: yesterday's results + today's upcoming (same calendar day only).
// After 18:00: full game night (tonight 22:00 through tomorrow morning).
func betsWindow() (from, to time.Time) {
	now := time.Now().In(almatyLoc)
	if testDate != nil {
		now = testDate.In(almatyLoc)
	}
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, almatyLoc)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, almatyLoc)
	if now.Hour() < 18 {
		from = noon.AddDate(0, 0, -1) // previous game night
		to = midnight                  // end of today — shows today's matches (e.g. 22:00), not tomorrow's
	} else {
		from = noon                   // current game night
		to = noon.AddDate(0, 0, 1)   // noon tomorrow
	}
	return
}

func matchDateKey(t time.Time) string {
	d := t.In(almatyLoc)
	return fmt.Sprintf("%d-%02d-%02d", d.Year(), d.Month(), d.Day())
}

func matchDateLabel(t time.Time) string {
	months := []string{
		"января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	d := t.In(almatyLoc)
	return fmt.Sprintf("%d %s", d.Day(), months[d.Month()-1])
}

// buildMatchRows builds inline keyboard rows for a list of matches,
// inserting non-clickable date headers when matches span multiple calendar days.
func buildMatchRows(matches []model.Match) (rows [][]tele.InlineButton, header string) {
	seen := make(map[int64]bool)

	type group struct {
		date    time.Time
		matches []model.Match
	}
	var groups []group
	dateIdx := map[string]int{}

	for _, m := range matches {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		key := matchDateKey(m.MatchDate)
		idx, ok := dateIdx[key]
		if !ok {
			idx = len(groups)
			d := m.MatchDate.In(almatyLoc)
			groups = append(groups, group{
				date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, almatyLoc),
			})
			dateIdx[key] = idx
		}
		groups[idx].matches = append(groups[idx].matches, m)
	}

	multiDay := len(groups) > 1
	now := time.Now().In(almatyLoc)
	today := fmt.Sprintf("%d-%02d-%02d", now.Year(), now.Month(), now.Day())
	tomorrow := fmt.Sprintf("%d-%02d-%02d", now.Year(), now.Month(), now.Day()+1)

	if !multiDay && len(groups) == 1 {
		key := matchDateKey(groups[0].date)
		switch key {
		case today:
			header = "Матчи сегодня:"
		case tomorrow:
			header = "Матчи завтра:"
		default:
			header = "Матчи " + matchDateLabel(groups[0].date) + ":"
		}
	} else {
		header = "Матчи:"
	}

	for _, g := range groups {
		if multiDay {
			rows = append(rows, []tele.InlineButton{
				{Text: "── " + matchDateLabel(g.date) + " ──", Data: "noop"},
			})
		}
		for _, m := range g.matches {
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
			rows = append(rows, []tele.InlineButton{
				{Text: label, Data: fmt.Sprintf("m|%d", m.ID)},
			})
		}
	}
	return
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

	rows, header := buildMatchRows(matches)
	return c.Send(header, &tele.ReplyMarkup{InlineKeyboard: rows})
}

func (h *Handler) handleBackToMatches(c tele.Context) error {
	ctx := context.Background()
	from, to := matchesWindow()
	matches, err := h.matches.GetUpcoming(ctx, from, to)
	if err != nil {
		return c.Respond()
	}

	if err := c.Respond(); err != nil {
		return err
	}

	if len(matches) == 0 {
		_, err := c.Bot().Edit(c.Message(), "Матчей нет.")
		return err
	}

	rows, header := buildMatchRows(matches)
	_, err = c.Bot().Edit(c.Message(), header, &tele.ReplyMarkup{InlineKeyboard: rows})
	return err
}
