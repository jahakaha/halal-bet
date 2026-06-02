package model

import "time"

const (
	TournamentBetWinner    = "winner"
	TournamentBetTopScorer = "top_scorer"

	PointsTournamentWinner    = 20
	PointsTournamentTopScorer = 15
)

type TournamentPrediction struct {
	ID        int64
	UserID    int64
	Type      string
	Value     string
	Points    *int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TournamentPredictionWithUser struct {
	TournamentPrediction
	Username string
}
