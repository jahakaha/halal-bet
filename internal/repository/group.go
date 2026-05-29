package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"halal-bet/internal/model"
)

type GroupRepository interface {
	Upsert(ctx context.Context, group *model.Group) error
	GetAll(ctx context.Context) ([]model.Group, error)
	GetByChatID(ctx context.Context, chatID int64) (*model.Group, error)
	GetByUserID(ctx context.Context, userID int64) ([]model.Group, error)
	AddMember(ctx context.Context, groupID, userID int64) error
	Leaderboard(ctx context.Context, groupID int64) ([]model.GroupLeaderboardEntry, error)
}

type groupRepository struct {
	db *pgxpool.Pool
}

func NewGroupRepository(db *pgxpool.Pool) GroupRepository {
	return &groupRepository{db: db}
}

func (r *groupRepository) GetAll(ctx context.Context) ([]model.Group, error) {
	sql, args, err := psql.
		Select("id", "telegram_chat_id", "name", "created_at").
		From("groups").
		OrderBy("created_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []model.Group
	for rows.Next() {
		var g model.Group
		if err := rows.Scan(&g.ID, &g.TelegramChatID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (r *groupRepository) Upsert(ctx context.Context, group *model.Group) error {
	sql, args, err := psql.
		Insert("groups").
		Columns("telegram_chat_id", "name").
		Values(group.TelegramChatID, group.Name).
		Suffix("ON CONFLICT (telegram_chat_id) DO UPDATE SET name = EXCLUDED.name RETURNING id, created_at").
		ToSql()
	if err != nil {
		return err
	}
	return r.db.QueryRow(ctx, sql, args...).Scan(&group.ID, &group.CreatedAt)
}

func (r *groupRepository) GetByChatID(ctx context.Context, chatID int64) (*model.Group, error) {
	sql, args, err := psql.
		Select("id", "telegram_chat_id", "name", "created_at").
		From("groups").
		Where("telegram_chat_id = ?", chatID).
		ToSql()
	if err != nil {
		return nil, err
	}
	var g model.Group
	err = r.db.QueryRow(ctx, sql, args...).Scan(&g.ID, &g.TelegramChatID, &g.Name, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *groupRepository) GetByUserID(ctx context.Context, userID int64) ([]model.Group, error) {
	sql, args, err := psql.
		Select("g.id", "g.telegram_chat_id", "g.name", "g.created_at").
		From("groups g").
		Join("group_members gm ON gm.group_id = g.id").
		Where("gm.user_id = ?", userID).
		OrderBy("g.created_at ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []model.Group
	for rows.Next() {
		var g model.Group
		if err := rows.Scan(&g.ID, &g.TelegramChatID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (r *groupRepository) AddMember(ctx context.Context, groupID, userID int64) error {
	sql, args, err := psql.
		Insert("group_members").
		Columns("group_id", "user_id").
		Values(groupID, userID).
		Suffix("ON CONFLICT DO NOTHING").
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, sql, args...)
	return err
}

func (r *groupRepository) Leaderboard(ctx context.Context, groupID int64) ([]model.GroupLeaderboardEntry, error) {
	query, args, err := psql.
		Select(
			"u.username",
			"COALESCE(SUM(p.points), 0) AS total_points",
			"COUNT(p.id) AS predictions_made",
		).
		From("group_members gm").
		Join("users u ON u.id = gm.user_id").
		LeftJoin("predictions p ON p.user_id = u.id AND p.points IS NOT NULL").
		Where("gm.group_id = ?", groupID).
		GroupBy("u.id", "u.username").
		OrderBy("total_points DESC", "u.username ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.GroupLeaderboardEntry
	for rows.Next() {
		var e model.GroupLeaderboardEntry
		if err := rows.Scan(&e.Username, &e.TotalPoints, &e.Predictions); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
