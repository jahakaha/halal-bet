package service

import (
	"context"
	"sort"

	"halal-bet/internal/model"
	"halal-bet/internal/repository"
)

type LeaderboardService struct {
	groups      repository.GroupRepository
	matches     repository.MatchRepository
	predictions repository.PredictionRepository
}

func NewLeaderboardService(
	groups repository.GroupRepository,
	matches repository.MatchRepository,
	predictions repository.PredictionRepository,
) *LeaderboardService {
	return &LeaderboardService{groups: groups, matches: matches, predictions: predictions}
}

// LiveLeaderboard returns the group leaderboard with live points from IN_PLAY matches added.
func (s *LeaderboardService) LiveLeaderboard(ctx context.Context, groupID int64) ([]model.GroupLeaderboardEntry, bool, error) {
	entries, err := s.groups.Leaderboard(ctx, groupID)
	if err != nil {
		return nil, false, err
	}

	inPlay, err := s.matches.GetInPlay(ctx)
	if err != nil || len(inPlay) == 0 {
		return entries, false, err
	}

	// Build username → index map for fast lookup.
	idx := make(map[string]int, len(entries))
	for i, e := range entries {
		idx[e.Username] = i
	}

	for _, m := range inPlay {
		preds, err := s.predictions.GetByMatchWithUsers(ctx, m.ID)
		if err != nil {
			continue
		}
		for _, pw := range preds {
			if pw.Points != nil {
				continue
			}
			pts := model.CalcPoints(&pw.Prediction, &m)
			if pts == nil {
				continue
			}
			i, ok := idx[pw.Username]
			if !ok {
				continue
			}
			entries[i].LivePoints += *pts
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Total() > entries[j].Total()
	})

	return entries, true, nil
}
