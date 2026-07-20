package repository

import (
	"context"

	sq "github.com/Masterminds/squirrel"
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
	GetMembersWithoutBet(ctx context.Context, groupID, matchID int64) ([]string, error)
	GetGroupStats(ctx context.Context, groupID int64) ([]model.UserStats, error)
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
	// Aggregate each source before joining. Joining predictions and
	// tournament_predictions directly would multiply rows for users that have
	// both kinds of bets, and therefore inflate their scores.
	const query = `
		SELECT
			u.username,
			COALESCE(mp.total_points, 0) + COALESCE(tp.total_points, 0) AS total_points,
			COALESCE(mp.predictions_made, 0) AS predictions_made
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		LEFT JOIN (
			SELECT user_id, COALESCE(SUM(points), 0) AS total_points, COUNT(id) AS predictions_made
			FROM predictions
			WHERE points IS NOT NULL
			GROUP BY user_id
		) mp ON mp.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COALESCE(SUM(points), 0) AS total_points
			FROM tournament_predictions
			WHERE points IS NOT NULL
			GROUP BY user_id
		) tp ON tp.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY total_points DESC, u.username ASC`

	rows, err := r.db.Query(ctx, query, groupID)
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

func (r *groupRepository) GetMembersWithoutBet(ctx context.Context, groupID, matchID int64) ([]string, error) {
	query, args, err := psql.
		Select("COALESCE(u.username, '')").
		From("group_members gm").
		Join("users u ON u.id = gm.user_id").
		Where("gm.group_id = ?", groupID).
		Where(sq.Expr("NOT EXISTS (SELECT 1 FROM predictions p WHERE p.user_id = u.id AND p.match_id = ?)", matchID)).
		OrderBy("u.username ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		usernames = append(usernames, username)
	}
	return usernames, rows.Err()
}

func (r *groupRepository) GetGroupStats(ctx context.Context, groupID int64) ([]model.UserStats, error) {
	const q = `
	SELECT
		u.username,
		COUNT(*)::int AS total,
		COUNT(*) FILTER (WHERE p.home_score = m.home_score AND p.away_score = m.away_score)::int AS exact_count,
		COUNT(*) FILTER (WHERE
			(p.home_score - p.away_score) = (m.home_score - m.away_score)
			AND NOT (p.home_score = m.home_score AND p.away_score = m.away_score)
		)::int AS diff_count,
		COUNT(*) FILTER (WHERE
			SIGN(p.home_score - p.away_score) = SIGN(m.home_score - m.away_score)
			AND (p.home_score - p.away_score) != (m.home_score - m.away_score)
		)::int AS outcome_count,
		COUNT(*) FILTER (WHERE p.bet_penalty)::int AS penalty_bets,
		COUNT(*) FILTER (WHERE p.bet_penalty AND m.had_penalty = true)::int AS penalty_hits,
		COUNT(*) FILTER (WHERE p.bet_red_card)::int AS redcard_bets,
		COUNT(*) FILTER (WHERE p.bet_red_card AND m.had_red_card = true)::int AS redcard_hits,
		COUNT(*) FILTER (WHERE p.bet_own_goal)::int AS owngoal_bets,
		COUNT(*) FILTER (WHERE p.bet_own_goal AND m.had_own_goal = true)::int AS owngoal_hits,
		COALESCE(SUM(p.points), 0) AS total_points,
		COUNT(*) FILTER (WHERE p.double_down)::int AS double_downs,
		COUNT(*) FILTER (WHERE p.double_down AND p.points > 0)::int AS double_down_hits
	FROM predictions p
	JOIN users u ON u.id = p.user_id
	JOIN wc2026_matches m ON m.id = p.match_id
	JOIN group_members gm ON gm.user_id = p.user_id AND gm.group_id = $1
	WHERE m.status = 'FINISHED'
	GROUP BY u.id, u.username
	ORDER BY total_points DESC, u.username ASC`

	rows, err := r.db.Query(ctx, q, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.UserStats
	for rows.Next() {
		var s model.UserStats
		if err := rows.Scan(
			&s.Username, &s.Total,
			&s.Exact, &s.Diff, &s.Outcome,
			&s.PenaltyBets, &s.PenaltyHits,
			&s.RedCardBets, &s.RedCardHits,
			&s.OwnGoalBets, &s.OwnGoalHits,
			&s.TotalPoints,
			&s.DoubleDowns, &s.DoubleDownHits,
		); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
