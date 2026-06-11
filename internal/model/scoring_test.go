package model

import (
	"testing"
)

func ptr(i int) *int { return &i }
func boolp(b bool) *bool { return &b }

func finished(home, away int) *Match {
	return &Match{
		Status:    MatchStatusFinished,
		HomeScore: ptr(home),
		AwayScore: ptr(away),
	}
}

func TestCalcPoints_NilScores(t *testing.T) {
	p := &Prediction{BetType: BetTypeExact, HomeScore: 2, AwayScore: 1}
	m := &Match{Status: MatchStatusInPlay}
	if CalcPoints(p, m) != nil {
		t.Error("expected nil when match scores are nil")
	}
}

func TestCalcPoints_Exact(t *testing.T) {
	cases := []struct {
		predHome, predAway int
		actualHome, actualAway int
		want int
	}{
		{2, 1, 2, 1, 5}, // exact
		{2, 0, 3, 1, 3}, // same diff (+2)
		{2, 1, 3, 2, 3}, // same diff (+1)
		{2, 0, 1, 0, 1}, // diff 2 ≠ 1, same outcome (home win) → 1
		{2, 1, 1, 0, 3}, // same diff (+1 both) → 3
		{2, 0, 3, 0, 1}, // diff 2 ≠ 3, same outcome (home win) → 1
		{1, 0, 2, 0, 1}, // diff 1 ≠ 2, same outcome (home win) → 1
		{0, 0, 1, 1, 3}, // same diff (0 = 0), both draw → 3
		{0, 1, 0, 2, 1}, // same outcome (away win)
		{2, 1, 0, 1, 0}, // wrong outcome
		{1, 0, 0, 1, 0}, // wrong outcome
	}

	for _, tc := range cases {
		p := &Prediction{BetType: BetTypeExact, HomeScore: tc.predHome, AwayScore: tc.predAway}
		m := finished(tc.actualHome, tc.actualAway)
		got := CalcPoints(p, m)
		if got == nil || *got != tc.want {
			t.Errorf("exact %d:%d vs actual %d:%d: want %d, got %v",
				tc.predHome, tc.predAway, tc.actualHome, tc.actualAway, tc.want, got)
		}
	}
}

func TestCalcPoints_Diff(t *testing.T) {
	cases := []struct {
		predDiff int
		actualHome, actualAway int
		want int
	}{
		{2, 3, 1, 3}, // diff matches (both +2)
		{2, 1, 3, 3}, // diff matches (both -2, stored as abs)
		{0, 1, 1, 3}, // draw matches
		{2, 1, 0, 0}, // diff mismatch
		{0, 2, 1, 0}, // draw predicted, home wins
	}

	for _, tc := range cases {
		p := &Prediction{BetType: BetTypeDiff, HomeScore: tc.predDiff}
		m := finished(tc.actualHome, tc.actualAway)
		got := CalcPoints(p, m)
		if got == nil || *got != tc.want {
			t.Errorf("diff %d vs actual %d:%d: want %d, got %v",
				tc.predDiff, tc.actualHome, tc.actualAway, tc.want, got)
		}
	}
}

func TestCalcPoints_Outcome(t *testing.T) {
	cases := []struct {
		predHome, predAway int
		actualHome, actualAway int
		want int
	}{
		{1, 0, 3, 1, 1}, // home win predicted and happened
		{0, 1, 0, 2, 1}, // away win
		{0, 0, 1, 1, 1}, // draw
		{1, 0, 0, 1, 0}, // home predicted, away won
		{0, 0, 2, 1, 0}, // draw predicted, home won
	}

	for _, tc := range cases {
		p := &Prediction{BetType: BetTypeOutcome, HomeScore: tc.predHome, AwayScore: tc.predAway}
		m := finished(tc.actualHome, tc.actualAway)
		got := CalcPoints(p, m)
		if got == nil || *got != tc.want {
			t.Errorf("outcome %d:%d vs actual %d:%d: want %d, got %v",
				tc.predHome, tc.predAway, tc.actualHome, tc.actualAway, tc.want, got)
		}
	}
}

func TestCalcPoints_DoubleDown(t *testing.T) {
	cases := []struct {
		betType            string
		predHome, predAway int
		actualHome, actualAway int
		want int
	}{
		{BetTypeExact, 2, 1, 2, 1, 10}, // exact 5 × 2
		{BetTypeExact, 2, 0, 3, 1, 6},  // diff 3 × 2
		{BetTypeOutcome, 1, 0, 2, 0, 2}, // outcome 1 × 2
		{BetTypeExact, 2, 1, 0, 1, -2}, // wrong → -2
		{BetTypeOutcome, 1, 0, 0, 1, -2}, // wrong outcome → -2
	}

	for _, tc := range cases {
		p := &Prediction{BetType: tc.betType, HomeScore: tc.predHome, AwayScore: tc.predAway, DoubleDown: true}
		m := finished(tc.actualHome, tc.actualAway)
		got := CalcPoints(p, m)
		if got == nil || *got != tc.want {
			t.Errorf("DD %s %d:%d vs %d:%d: want %d, got %v",
				tc.betType, tc.predHome, tc.predAway, tc.actualHome, tc.actualAway, tc.want, got)
		}
	}
}

func TestCalcPoints_RiskyBets(t *testing.T) {
	base := func() *Prediction {
		return &Prediction{BetType: BetTypeExact, HomeScore: 2, AwayScore: 1}
	}
	m := finished(2, 1)

	t.Run("penalty hit", func(t *testing.T) {
		p := base()
		p.BetPenalty = true
		m.HadPenalty = boolp(true)
		got := CalcPoints(p, m)
		if *got != 7 { // 5 + 2
			t.Errorf("want 7, got %d", *got)
		}
	})

	t.Run("penalty miss", func(t *testing.T) {
		p := base()
		p.BetPenalty = true
		m.HadPenalty = boolp(false)
		got := CalcPoints(p, m)
		if *got != 4 { // 5 - 1
			t.Errorf("want 4, got %d", *got)
		}
	})

	t.Run("red card hit", func(t *testing.T) {
		p := base()
		p.BetRedCard = true
		m.HadPenalty = nil
		m.HadRedCard = boolp(true)
		got := CalcPoints(p, m)
		if *got != 8 { // 5 + 3
			t.Errorf("want 8, got %d", *got)
		}
	})

	t.Run("own goal hit", func(t *testing.T) {
		p := base()
		p.BetOwnGoal = true
		m.HadRedCard = nil
		m.HadOwnGoal = boolp(true)
		got := CalcPoints(p, m)
		if *got != 10 { // 5 + 5
			t.Errorf("want 10, got %d", *got)
		}
	})

	t.Run("risky nil = ignored", func(t *testing.T) {
		p := base()
		p.BetPenalty = true
		m.HadOwnGoal = nil
		m.HadPenalty = nil // not yet determined
		got := CalcPoints(p, m)
		if *got != 5 { // risky not counted when nil
			t.Errorf("want 5, got %d", *got)
		}
	})
}

func TestCalcPoints_DoubleDownPlusRisky(t *testing.T) {
	// exact match 5 → DD → 10, penalty hit +2 = 12
	p := &Prediction{
		BetType:    BetTypeExact,
		HomeScore:  2,
		AwayScore:  1,
		DoubleDown: true,
		BetPenalty: true,
	}
	m := finished(2, 1)
	m.HadPenalty = boolp(true)
	got := CalcPoints(p, m)
	if *got != 12 {
		t.Errorf("want 12, got %d", *got)
	}
}

func TestOutcomeOf(t *testing.T) {
	if OutcomeOf(2, 1) != OutcomeHome { t.Error("2:1 should be home win") }
	if OutcomeOf(0, 1) != OutcomeAway { t.Error("0:1 should be away win") }
	if OutcomeOf(1, 1) != OutcomeDraw { t.Error("1:1 should be draw") }
	if OutcomeOf(0, 0) != OutcomeDraw { t.Error("0:0 should be draw") }
}
