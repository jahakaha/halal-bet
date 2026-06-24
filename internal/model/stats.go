package model

type UserStats struct {
	Username    string
	Total       int
	Exact       int
	Diff        int
	Outcome     int
	PenaltyBets int
	PenaltyHits int
	RedCardBets int
	RedCardHits int
	OwnGoalBets int
	OwnGoalHits int
	TotalPoints int
}

func (s *UserStats) Miss() int {
	return s.Total - s.Exact - s.Diff - s.Outcome
}
