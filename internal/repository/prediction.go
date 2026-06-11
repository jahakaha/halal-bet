package repository

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"halal-bet/internal/model"
)

type PredictionRepository interface {
	Upsert(ctx context.Context, p *model.Prediction) error
	GetByUserAndMatch(ctx context.Context, userID, matchID int64) (*model.Prediction, error)
	GetByMatch(ctx context.Context, matchID int64) ([]model.Prediction, error)
	GetByMatchWithUsers(ctx context.Context, matchID int64) ([]model.PredictionWithUser, error)
	CountDoubleDowns(ctx context.Context, userID, excludeMatchID int64) (int, error)
	UpdatePoints(ctx context.Context, matchID int64, points map[int64]int) error
}

type predictionRepository struct {
	db *pgxpool.Pool
}

func NewPredictionRepository(db *pgxpool.Pool) PredictionRepository {
	return &predictionRepository{db: db}
}

func (r *predictionRepository) Upsert(ctx context.Context, p *model.Prediction) error {
	sql, args, err := psql.
		Insert("predictions").
		Columns("user_id", "match_id", "bet_type", "home_score", "away_score", "double_down", "bet_penalty", "bet_red_card", "bet_own_goal").
		Values(p.UserID, p.MatchID, p.BetType, p.HomeScore, p.AwayScore, p.DoubleDown, p.BetPenalty, p.BetRedCard, p.BetOwnGoal).
		Suffix(`ON CONFLICT (user_id, match_id) DO UPDATE SET
			bet_type     = EXCLUDED.bet_type,
			home_score   = EXCLUDED.home_score,
			away_score   = EXCLUDED.away_score,
			double_down  = EXCLUDED.double_down,
			bet_penalty  = EXCLUDED.bet_penalty,
			bet_red_card = EXCLUDED.bet_red_card,
			bet_own_goal = EXCLUDED.bet_own_goal,
			updated_at   = NOW()
		RETURNING id, created_at, updated_at`).
		ToSql()
	if err != nil {
		return err
	}
	return r.db.QueryRow(ctx, sql, args...).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *predictionRepository) GetByUserAndMatch(ctx context.Context, userID, matchID int64) (*model.Prediction, error) {
	sql, args, err := psql.
		Select("id", "user_id", "match_id", "bet_type", "home_score", "away_score", "double_down", "bet_penalty", "bet_red_card", "bet_own_goal", "points", "created_at", "updated_at").
		From("predictions").
		Where("user_id = ? AND match_id = ?", userID, matchID).
		ToSql()
	if err != nil {
		return nil, err
	}
	var p model.Prediction
	err = r.db.QueryRow(ctx, sql, args...).Scan(
		&p.ID, &p.UserID, &p.MatchID, &p.BetType, &p.HomeScore, &p.AwayScore,
		&p.DoubleDown, &p.BetPenalty, &p.BetRedCard, &p.BetOwnGoal,
		&p.Points, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *predictionRepository) GetByMatch(ctx context.Context, matchID int64) ([]model.Prediction, error) {
	sql, args, err := psql.
		Select("id", "user_id", "match_id", "bet_type", "home_score", "away_score", "double_down", "bet_penalty", "bet_red_card", "bet_own_goal", "points", "created_at", "updated_at").
		From("predictions").
		Where("match_id = ?", matchID).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var predictions []model.Prediction
	for rows.Next() {
		var p model.Prediction
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.MatchID, &p.BetType, &p.HomeScore, &p.AwayScore,
			&p.DoubleDown, &p.BetPenalty, &p.BetRedCard, &p.BetOwnGoal,
			&p.Points, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		predictions = append(predictions, p)
	}
	return predictions, rows.Err()
}

func (r *predictionRepository) GetByMatchWithUsers(ctx context.Context, matchID int64) ([]model.PredictionWithUser, error) {
	sql, args, err := psql.
		Select("p.id", "p.user_id", "p.match_id", "p.bet_type", "p.home_score", "p.away_score",
			"p.double_down", "p.bet_penalty", "p.bet_red_card", "p.bet_own_goal", "p.points",
			"p.created_at", "p.updated_at", "u.username").
		From("predictions p").
		Join("users u ON u.id = p.user_id").
		Where("p.match_id = ?", matchID).
		OrderBy("u.username ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.PredictionWithUser
	for rows.Next() {
		var pw model.PredictionWithUser
		if err := rows.Scan(
			&pw.ID, &pw.UserID, &pw.MatchID, &pw.BetType, &pw.HomeScore, &pw.AwayScore,
			&pw.DoubleDown, &pw.BetPenalty, &pw.BetRedCard, &pw.BetOwnGoal, &pw.Points,
			&pw.CreatedAt, &pw.UpdatedAt, &pw.Username,
		); err != nil {
			return nil, err
		}
		result = append(result, pw)
	}
	return result, rows.Err()
}

func (r *predictionRepository) CountDoubleDowns(ctx context.Context, userID, excludeMatchID int64) (int, error) {
	b := psql.Select("COUNT(*)").From("predictions").Where("user_id = ? AND double_down = true", userID)
	if excludeMatchID != 0 {
		b = b.Where("match_id != ?", excludeMatchID)
	}
	sql, args, err := b.ToSql()
	if err != nil {
		return 0, err
	}
	var count int
	err = r.db.QueryRow(ctx, sql, args...).Scan(&count)
	return count, err
}

// UpdatePoints bulk-updates points for all predictions of a finished match.
// points is a map of predictionID → calculated points.
func (r *predictionRepository) UpdatePoints(ctx context.Context, matchID int64, points map[int64]int) error {
	if len(points) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for predID, pts := range points {
		sql, args, err := psql.
			Update("predictions").
			Set("points", pts).
			Set("updated_at", sq.Expr("NOW()")).
			Where("id = ? AND match_id = ?", predID, matchID).
			ToSql()
		if err != nil {
			return err
		}
		batch.Queue(sql, args...)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range points {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}
