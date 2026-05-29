package repository

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"

	"halal-bet/internal/model"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type UserRepository interface {
	CreateIfNotExist(ctx context.Context, user *model.User) error
	GetIDByTelegramID(ctx context.Context, telegramID int64) (int64, error)
}

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetIDByTelegramID(ctx context.Context, telegramID int64) (int64, error) {
	sql, args, err := psql.
		Select("id").
		From("users").
		Where("telegram_id = ?", telegramID).
		ToSql()
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRow(ctx, sql, args...).Scan(&id)
	return id, err
}

func (r *userRepository) CreateIfNotExist(ctx context.Context, user *model.User) error {
	sql, args, err := psql.
		Insert("users").
		Columns("telegram_id", "username", "created_at").
		Values(user.TelegramID, user.Username, user.CreatedAt).
		Suffix("ON CONFLICT (telegram_id) DO NOTHING").
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, sql, args...)
	return err
}
