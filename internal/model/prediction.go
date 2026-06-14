package model

import "time"

const (
	BetTypeExact   = "exact"
	BetTypeDiff    = "diff"
	BetTypeOutcome = "outcome"
)

type Prediction struct {
	ID         int64     `db:"id"`
	UserID     int64     `db:"user_id"`
	MatchID    int64     `db:"match_id"`
	BetType    string    `db:"bet_type"`
	HomeScore  int       `db:"home_score"`
	AwayScore  int       `db:"away_score"`
	DoubleDown bool      `db:"double_down"`
	BetPenalty bool      `db:"bet_penalty"`
	BetRedCard bool      `db:"bet_red_card"`
	BetOwnGoal bool      `db:"bet_own_goal"`
	Points     *int      `db:"points"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

type PredictionWithUser struct {
	Prediction
	Username string
}

type PredictionWithMatch struct {
	Prediction
	HomeTeam        string
	AwayTeam        string
	MatchDate       time.Time
	MatchStatus     MatchStatus
	ActualHomeScore *int
	ActualAwayScore *int
	HadPenalty      *bool
	HadRedCard      *bool
	HadOwnGoal      *bool
}

// Outcome returns the predicted match outcome.
func (p *Prediction) Outcome() Outcome {
	return OutcomeOf(p.HomeScore, p.AwayScore)
}

type Outcome int8

const (
	OutcomeHome Outcome = 1
	OutcomeDraw Outcome = 0
	OutcomeAway Outcome = -1
)

func OutcomeOf(home, away int) Outcome {
	switch {
	case home > away:
		return OutcomeHome
	case home < away:
		return OutcomeAway
	default:
		return OutcomeDraw
	}
}
