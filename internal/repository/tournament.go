package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"halal-bet/internal/model"
)

type TournamentRepository interface {
	Upsert(ctx context.Context, userID int64, betType, value string) error
	GetByUser(ctx context.Context, userID int64) ([]model.TournamentPrediction, error)
	GetAll(ctx context.Context) ([]model.TournamentPredictionWithUser, error)
	UpdatePoints(ctx context.Context, betType, value string, points int) error
}

type tournamentRepository struct {
	db *pgxpool.Pool
}

func NewTournamentRepository(db *pgxpool.Pool) TournamentRepository {
	return &tournamentRepository{db: db}
}

func (r *tournamentRepository) Upsert(ctx context.Context, userID int64, betType, value string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO tournament_predictions (user_id, type, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, type) DO UPDATE
		SET value = EXCLUDED.value, updated_at = NOW()
	`, userID, betType, value)
	return err
}

func (r *tournamentRepository) GetByUser(ctx context.Context, userID int64) ([]model.TournamentPrediction, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, type, value, points, created_at, updated_at
		FROM tournament_predictions
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.TournamentPrediction
	for rows.Next() {
		var p model.TournamentPrediction
		if err := rows.Scan(&p.ID, &p.UserID, &p.Type, &p.Value, &p.Points, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *tournamentRepository) GetAll(ctx context.Context) ([]model.TournamentPredictionWithUser, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tp.id, tp.user_id, tp.type, tp.value, tp.points, tp.created_at, tp.updated_at,
		       COALESCE(u.username, '') AS username
		FROM tournament_predictions tp
		JOIN users u ON u.id = tp.user_id
		ORDER BY u.username, tp.type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.TournamentPredictionWithUser
	for rows.Next() {
		var p model.TournamentPredictionWithUser
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Type, &p.Value, &p.Points,
			&p.CreatedAt, &p.UpdatedAt, &p.Username,
		); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *tournamentRepository) UpdatePoints(ctx context.Context, betType, value string, points int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tournament_predictions
		SET points = $3
		WHERE type = $1 AND LOWER(value) = LOWER($2)
	`, betType, value, points)
	return err
}
