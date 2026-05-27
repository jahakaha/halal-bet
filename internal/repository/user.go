package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"halal-bet/internal/model"
)

type UserRepository interface {
	CreateIfNotExist(ctx context.Context, user *model.User) error
}

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) CreateIfNotExist(ctx context.Context, user *model.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (telegram_id, username, created_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (telegram_id) DO NOTHING`,
		user.TelegramID, user.Username, user.CreatedAt,
	)
	return err
}
