package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
	"halal-bet/internal/repository"
	"halal-bet/internal/util"
)

var almatyLoc = mustLoadLocation("Asia/Almaty")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

type NotificationService struct {
	bot     *tele.Bot
	groups  repository.GroupRepository
	matches repository.MatchRepository
}

func NewNotificationService(bot *tele.Bot, groups repository.GroupRepository, matches repository.MatchRepository) *NotificationService {
	return &NotificationService{bot: bot, groups: groups, matches: matches}
}

// SendDailyResults sends two messages at 13:00 Almaty:
//  1. Yesterday's match results (scores)
//  2. Totalizator leaderboard per chat group
func (s *NotificationService) SendDailyResults(ctx context.Context, now time.Time) {
	now = now.In(almatyLoc)
	yesterday := now.AddDate(0, 0, -1)
	from := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, almatyLoc).UTC()
	to := from.Add(24 * time.Hour)

	finished, err := s.matches.GetFinishedInWindow(ctx, from, to)
	if err != nil {
		return
	}

	chatGroups, err := s.groups.GetAll(ctx)
	if err != nil || len(chatGroups) == 0 {
		return
	}

	// Message 1: match results (same for all chat groups)
	if len(finished) > 0 {
		resultsMsg := formatMatchResults(yesterday, finished)
		for _, g := range chatGroups {
			chat := &tele.Chat{ID: g.TelegramChatID}
			s.bot.Send(chat, resultsMsg, tele.ModeMarkdown) //nolint:errcheck
		}
	}

	// Message 2: totalizator leaderboard (per chat group)
	for _, g := range chatGroups {
		entries, err := s.groups.Leaderboard(ctx, g.ID)
		if err != nil || len(entries) == 0 {
			continue
		}
		msg := formatLeaderboard(g.Name, entries)
		chat := &tele.Chat{ID: g.TelegramChatID}
		s.bot.Send(chat, msg, tele.ModeMarkdown) //nolint:errcheck
	}
}

// SendDailyMatches sends two messages at 20:00 Almaty:
//  1. WC2026 group standings for groups that have matches tomorrow
//  2. Tomorrow's matches with inline betting buttons
func (s *NotificationService) SendDailyMatches(ctx context.Context, now time.Time) {
	now = now.In(almatyLoc)
	tomorrow := now.AddDate(0, 0, 1)
	from := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, almatyLoc).UTC()
	to := from.Add(24 * time.Hour)

	matches, err := s.matches.GetUpcoming(ctx, from, to)
	if err != nil || len(matches) == 0 {
		return
	}

	chatGroups, err := s.groups.GetAll(ctx)
	if err != nil || len(chatGroups) == 0 {
		return
	}

	botUsername := s.bot.Me.Username
	standingsMsg := s.buildGroupStandingsMsg(ctx, matches)
	matchesMsg, markup := formatTomorrowMatches(tomorrow, matches, botUsername)

	for _, g := range chatGroups {
		chat := &tele.Chat{ID: g.TelegramChatID}
		if standingsMsg != "" {
			s.bot.Send(chat, standingsMsg, tele.ModeMarkdown) //nolint:errcheck
		}
		s.bot.Send(chat, matchesMsg, markup) //nolint:errcheck
	}
}

// buildGroupStandingsMsg collects unique groups from tomorrow's matches
// and builds a single standings message covering all of them.
func (s *NotificationService) buildGroupStandingsMsg(ctx context.Context, matches []model.Match) string {
	seen := map[string]bool{}
	var groupNames []string
	for _, m := range matches {
		if m.Group != nil && !seen[*m.Group] {
			seen[*m.Group] = true
			groupNames = append(groupNames, *m.Group)
		}
	}
	if len(groupNames) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, name := range groupNames {
		entries, err := s.matches.GetGroupStandings(ctx, name)
		if err != nil || len(entries) == 0 {
			continue
		}
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(formatGroupStandings(name, entries))
	}
	return sb.String()
}

// ── formatters ────────────────────────────────────────────────────────────────

func formatMatchResults(date time.Time, matches []model.Match) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚽️ *Результаты %s*\n\n", formatDate(date)))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf(
			"%s — %s  *%s : %s*\n",
			util.WithFlag(m.HomeTeam), util.WithFlag(m.AwayTeam),
			scoreStr(m.HomeScore), scoreStr(m.AwayScore),
		))
	}
	return sb.String()
}

func formatGroupStandings(groupName string, entries []model.StandingEntry) string {
	label := groupLabel(groupName)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 *%s*\n", label))
	sb.WriteString("```\n")
	sb.WriteString(" #  Команда            И  В  Н  П   О\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("%2d  %-18s %d  %d  %d  %d  %2d\n",
			i+1, e.Team, e.Played, e.Won, e.Drawn, e.Lost, e.Points))
	}
	sb.WriteString("```")
	return sb.String()
}

func formatTomorrowMatches(date time.Time, matches []model.Match, botUsername string) (string, *tele.ReplyMarkup) {
	header := fmt.Sprintf("📅 *Матчи %s*\n\nСделать ставку 👇", formatDate(date))
	rows := make([][]tele.InlineButton, 0, len(matches))
	for _, m := range matches {
		localTime := m.MatchDate.In(almatyLoc).Format("15:04")
		label := fmt.Sprintf("%s — %s  %s", util.WithFlag(m.HomeTeam), util.WithFlag(m.AwayTeam), localTime)
		btn := tele.InlineButton{
			Text: label,
			URL:  fmt.Sprintf("https://t.me/%s?start=m_%d", botUsername, m.ID),
		}
		rows = append(rows, []tele.InlineButton{btn})
	}
	return header, &tele.ReplyMarkup{InlineKeyboard: rows}
}

func formatLeaderboard(groupName string, entries []model.GroupLeaderboardEntry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🏆 *Тотализатор — %s*\n\n", groupName))
	medals := []string{"🥇", "🥈", "🥉"}
	for i, e := range entries {
		rank := fmt.Sprintf("%d.", i+1)
		if i < len(medals) {
			rank = medals[i]
		}
		name := e.Username
		if name == "" {
			name = "Аноним"
		}
		sb.WriteString(fmt.Sprintf("%s *%s* — %d очков (%d ставок)\n",
			rank, name, e.TotalPoints, e.Predictions))
	}
	return sb.String()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scoreStr(s *int) string {
	if s == nil {
		return "?"
	}
	return fmt.Sprintf("%d", *s)
}

func formatDate(t time.Time) string {
	months := []string{
		"января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	d := t.In(almatyLoc)
	return fmt.Sprintf("%d %s", d.Day(), months[d.Month()-1])
}

// groupLabel converts "GROUP_A" → "Группа A", leaves knockout stage names as-is.
func groupLabel(name string) string {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "GROUP_") {
		return "Группа " + name[len("GROUP_"):]
	}
	return strings.ReplaceAll(name, "_", " ")
}
