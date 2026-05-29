package bot

import (
	"testing"

	"halal-bet/internal/model"
)

func TestPointWord(t *testing.T) {
	cases := []struct{ n int; want string }{
		{0, "очков"}, {1, "очко"}, {2, "очка"}, {4, "очка"}, {5, "очков"}, {11, "очков"}, {-1, "очков"},
	}
	for _, tc := range cases {
		if got := pointWord(tc.n); got != tc.want {
			t.Errorf("pointWord(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestBetWord(t *testing.T) {
	cases := []struct{ n int; want string }{
		{0, "ставок"}, {1, "ставка"}, {2, "ставки"}, {4, "ставки"}, {5, "ставок"},
	}
	for _, tc := range cases {
		if got := betWord(tc.n); got != tc.want {
			t.Errorf("betWord(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestBuildLeaderboardMsg_Empty(t *testing.T) {
	msg := buildLeaderboardMsg(nil, false)
	if msg != "Ещё никто не делал ставок." {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestBuildLeaderboardMsg_LiveIndicator(t *testing.T) {
	entries := []model.GroupLeaderboardEntry{
		{Username: "user1", TotalPoints: 10},
	}
	msg := buildLeaderboardMsg(entries, true)
	if !contains(msg, "🟢") {
		t.Error("live message should contain 🟢")
	}

	msg = buildLeaderboardMsg(entries, false)
	if contains(msg, "🟢") {
		t.Error("non-live message should not contain 🟢")
	}
}

func TestBuildLeaderboardMsg_LivePoints(t *testing.T) {
	entries := []model.GroupLeaderboardEntry{
		{Username: "user1", TotalPoints: 10, LivePoints: 3},
		{Username: "user2", TotalPoints: 8, LivePoints: 0},
	}
	msg := buildLeaderboardMsg(entries, true)
	if !contains(msg, "+3 в игре") {
		t.Errorf("expected live points annotation, got: %s", msg)
	}
	// user2 has no live points — no annotation
	if contains(msg, "+0 в игре") {
		t.Errorf("zero live points should not be shown: %s", msg)
	}
}

func TestBuildLeaderboardMsg_Medals(t *testing.T) {
	entries := []model.GroupLeaderboardEntry{
		{Username: "gold"},
		{Username: "silver"},
		{Username: "bronze"},
		{Username: "fourth"},
	}
	msg := buildLeaderboardMsg(entries, false)
	for _, medal := range []string{"🥇", "🥈", "🥉"} {
		if !contains(msg, medal) {
			t.Errorf("expected medal %s in message", medal)
		}
	}
	if !contains(msg, "4.") {
		t.Error("expected rank 4. for fourth place")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
