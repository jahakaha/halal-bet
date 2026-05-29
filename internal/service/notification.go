package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"halal-bet/internal/model"
	"halal-bet/internal/repository"
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
	bot    *tele.Bot
	groups repository.GroupRepository
	matches repository.MatchRepository
}

func NewNotificationService(bot *tele.Bot, groups repository.GroupRepository, matches repository.MatchRepository) *NotificationService {
	return &NotificationService{bot: bot, groups: groups, matches: matches}
}

// SendDailyMatches отправляет расписание матчей на сегодня во все группы (20:00 алм.)
func (s *NotificationService) SendDailyMatches(ctx context.Context) {
	now := time.Now().In(almatyLoc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, almatyLoc).UTC()
	to := from.Add(24 * time.Hour)

	todayMatches, err := s.matches.GetUpcoming(ctx, from, to)
	if err != nil || len(todayMatches) == 0 {
		return
	}

	msg := formatDailyMatches(todayMatches)
	s.broadcast(ctx, msg)
}

// SendDailyResults отправляет таблицу каждой группы (13:00 алм.)
func (s *NotificationService) SendDailyResults(ctx context.Context) {
	groups, err := s.groups.GetAll(ctx)
	if err != nil || len(groups) == 0 {
		return
	}

	for _, g := range groups {
		entries, err := s.groups.Leaderboard(ctx, g.ID)
		if err != nil || len(entries) == 0 {
			continue
		}
		msg := formatLeaderboard(g.Name, entries)
		chat := &tele.Chat{ID: g.TelegramChatID}
		s.bot.Send(chat, msg, tele.ModeMarkdown) //nolint:errcheck
	}
}

func (s *NotificationService) broadcast(ctx context.Context, msg string) {
	groups, err := s.groups.GetAll(ctx)
	if err != nil {
		return
	}
	for _, g := range groups {
		chat := &tele.Chat{ID: g.TelegramChatID}
		s.bot.Send(chat, msg, tele.ModeMarkdown) //nolint:errcheck
	}
}

func formatDailyMatches(matches []model.Match) string {
	var sb strings.Builder
	sb.WriteString("*Матчи сегодня*\n\n")

	for _, m := range matches {
		localTime := m.MatchDate.In(almatyLoc).Format("15:04")
		group := ""
		if m.Group != nil {
			group = fmt.Sprintf(" · %s", strings.ReplaceAll(*m.Group, "_", " "))
		}
		sb.WriteString(fmt.Sprintf("%s  %s — %s%s\n", localTime, m.HomeTeam, m.AwayTeam, group))
	}

	sb.WriteString("\nСделать ставку: /matches")
	return sb.String()
}

func formatLeaderboard(groupName string, entries []model.GroupLeaderboardEntry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🏆 *Таблица — %s*\n\n", groupName))

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
