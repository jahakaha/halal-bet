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
	mu     sync.Mutex
	states map[int64]*predictionState
}

func newStateStore() *stateStore {
	return &stateStore{states: make(map[int64]*predictionState)}
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
