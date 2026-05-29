package model

import "time"

type Group struct {
	ID             int64     `db:"id"`
	TelegramChatID int64     `db:"telegram_chat_id"`
	Name           string    `db:"name"`
	CreatedAt      time.Time `db:"created_at"`
}

type GroupMember struct {
	GroupID  int64     `db:"group_id"`
	UserID   int64     `db:"user_id"`
	JoinedAt time.Time `db:"joined_at"`
}

// GroupLeaderboardEntry — one row in the leaderboard for a group.
type GroupLeaderboardEntry struct {
	Username    string `db:"username"`
	TotalPoints int    `db:"total_points"`
	Predictions int    `db:"predictions_made"`
	LivePoints  int    // in-memory: potential points from IN_PLAY matches
}

func (e GroupLeaderboardEntry) Total() int { return e.TotalPoints + e.LivePoints }
