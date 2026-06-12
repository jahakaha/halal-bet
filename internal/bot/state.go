package bot

import "sync"

type specialBet string

const (
	specialNone    specialBet = ""
	specialPenalty specialBet = "penalty"
	specialRedCard specialBet = "red_card"
	specialOwnGoal specialBet = "own_goal"
)

type predictionState struct {
	matchID     int64
	groupChatID int64 // Telegram chat ID of the group the user came from (0 if unknown)
	homeTeam    string
	awayTeam    string
	betType     string
	homeScore   int
	awayScore   int
	special     specialBet
	doubleDown  bool
	ddRemaining int
	msgID       int
}

type stateStore struct {
	mu         sync.Mutex
	states     map[int64]*predictionState
	tournament map[int64]string // userID → betType ("winner" | "top_scorer")
}

func newStateStore() *stateStore {
	return &stateStore{
		states:     make(map[int64]*predictionState),
		tournament: make(map[int64]string),
	}
}

func (s *stateStore) set(userID int64, st *predictionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[userID] = st
}

func (s *stateStore) get(userID int64) (*predictionState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[userID]
	return st, ok
}

func (s *stateStore) del(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, userID)
}

func (s *stateStore) setTournament(userID int64, betType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tournament[userID] = betType
}

func (s *stateStore) getTournament(userID int64) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tournament[userID]
	return t, ok
}

func (s *stateStore) clearTournament(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tournament, userID)
}
