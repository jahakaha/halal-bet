package model

import "time"

type MatchStatus string

const (
	MatchStatusTimed     MatchStatus = "TIMED"
	MatchStatusInPlay    MatchStatus = "IN_PLAY"
	MatchStatusFinished  MatchStatus = "FINISHED"
	MatchStatusPostponed MatchStatus = "POSTPONED"
)

type StandingEntry struct {
	Team   string
	Played int
	Won    int
	Drawn  int
	Lost   int
	GF     int
	GA     int
	Points int
}

func (s StandingEntry) GD() int { return s.GF - s.GA }

type Match struct {
	ID           int64       `db:"id"`
	ExternalID   int64       `db:"external_id"`
	SofascoreID  *int64      `db:"sofascore_id"`
	HomeTeam     string      `db:"home_team"`
	AwayTeam     string      `db:"away_team"`
	MatchDate    time.Time   `db:"match_date"`
	Status       MatchStatus `db:"status"`
	HomeScore    *int        `db:"home_score"`
	AwayScore    *int        `db:"away_score"`
	Stage        string      `db:"stage"`
	Group        *string     `db:"group_name"`
	Matchday     *int        `db:"matchday"`
	HadRedCard   *bool       `db:"had_red_card"`
	HadPenalty   *bool       `db:"had_penalty"`
	HadOwnGoal   *bool       `db:"had_own_goal"`
	UpdatedAt    time.Time   `db:"updated_at"`
}
