package model

const DoubleDownLimit = 5

// CalcPoints calculates points for a prediction against actual match result.
// Returns nil if the match is not finished yet.
func CalcPoints(p *Prediction, m *Match) *int {
	if m.HomeScore == nil || m.AwayScore == nil {
		return nil
	}

	pts := basePoints(p, *m.HomeScore, *m.AwayScore)

	if p.DoubleDown {
		if pts > 0 {
			pts *= 2
		} else {
			pts = -2
		}
	}

	pts += riskyPoints(p, m)

	return &pts
}

func basePoints(p *Prediction, actualHome, actualAway int) int {
	if p.HomeScore == actualHome && p.AwayScore == actualAway {
		return 5
	}
	if p.HomeScore-p.AwayScore == actualHome-actualAway {
		return 3
	}
	if OutcomeOf(p.HomeScore, p.AwayScore) == OutcomeOf(actualHome, actualAway) {
		return 1
	}
	return 0
}

func riskyPoints(p *Prediction, m *Match) int {
	pts := 0
	if p.BetPenalty && m.HadPenalty != nil {
		if *m.HadPenalty {
			pts += 2
		} else {
			pts -= 1
		}
	}
	if p.BetRedCard && m.HadRedCard != nil {
		if *m.HadRedCard {
			pts += 3
		} else {
			pts -= 2
		}
	}

	if p.BetOwnGoal && m.HadOwnGoal != nil {
		if *m.HadOwnGoal {
			pts += 5
		} else {
			pts -= 3
		}
	}

	return pts
}
