package model

import "time"

const (
	TournamentBetWinner    = "winner"
	TournamentBetTopScorer = "top_scorer"

	PointsTournamentWinner    = 20
	PointsTournamentTopScorer = 15
)

type TournamentPrediction struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Points    *int      `json:"points"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TournamentPredictionWithUser struct {
	TournamentPrediction
	Username string `json:"username"`
}

// TournamentPredictionSummary is a user's two tournament choices and the
// points they earned from them.
type TournamentPredictionSummary struct {
	Username        string `json:"username"`
	Team            string `json:"team"`
	TeamPoints      int    `json:"team_points"`
	TopScorer       string `json:"top_scorer"`
	TopScorerPoints int    `json:"top_scorer_points"`
}
