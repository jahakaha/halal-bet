package repository

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"

	"halal-bet/internal/model"
)

type MatchRepository interface {
	Upsert(ctx context.Context, matches []model.Match) error
	GetByID(ctx context.Context, id int64) (*model.Match, error)
	GetUpcoming(ctx context.Context, from, to time.Time) ([]model.Match, error)
	GetFinishedInWindow(ctx context.Context, from, to time.Time) ([]model.Match, error)
	GetFinishedWithPendingScores(ctx context.Context) ([]model.Match, error)
	GetInPlay(ctx context.Context) ([]model.Match, error)
	GetFinishedForEventSync(ctx context.Context) ([]model.Match, error)
	UpdateEvents(ctx context.Context, matchID, sofascoreID int64, hadRedCard, hadPenalty, hadOwnGoal bool) error
	GetGroupStandings(ctx context.Context, groupName string) ([]model.StandingEntry, error)
}

type matchRepository struct {
	db *pgxpool.Pool
}

func NewMatchRepository(db *pgxpool.Pool) MatchRepository {
	return &matchRepository{db: db}
}

func (r *matchRepository) Upsert(ctx context.Context, matches []model.Match) error {
	if len(matches) == 0 {
		return nil
	}

	q := psql.
		Insert("wc2026_matches").
		Columns("external_id", "home_team", "away_team", "match_date", "status", "home_score", "away_score", "stage", "group_name", "matchday", "updated_at").
		Suffix(`ON CONFLICT (external_id) DO UPDATE SET
			home_team   = EXCLUDED.home_team,
			away_team   = EXCLUDED.away_team,
			match_date  = EXCLUDED.match_date,
			status      = EXCLUDED.status,
			home_score  = EXCLUDED.home_score,
			away_score  = EXCLUDED.away_score,
			stage       = EXCLUDED.stage,
			group_name  = EXCLUDED.group_name,
			matchday    = EXCLUDED.matchday,
			updated_at  = EXCLUDED.updated_at`)

	for _, m := range matches {
		q = q.Values(
			m.ExternalID,
			m.HomeTeam,
			m.AwayTeam,
			m.MatchDate.UTC(),
			m.Status,
			m.HomeScore,
			m.AwayScore,
			m.Stage,
			m.Group,
			m.Matchday,
			time.Now().UTC(),
		)
	}

	sql, args, err := q.ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, sql, args...)
	return err
}

func (r *matchRepository) GetByID(ctx context.Context, id int64) (*model.Match, error) {
	sql, args, err := psql.
		Select("id", "external_id", "home_team", "away_team", "match_date", "status", "home_score", "away_score", "stage", "group_name", "matchday", "had_red_card", "had_penalty", "had_own_goal", "updated_at").
		From("wc2026_matches").
		Where("id = ?", id).
		ToSql()
	if err != nil {
		return nil, err
	}
	var m model.Match
	err = r.db.QueryRow(ctx, sql, args...).Scan(
		&m.ID, &m.ExternalID, &m.HomeTeam, &m.AwayTeam,
		&m.MatchDate, &m.Status, &m.HomeScore, &m.AwayScore,
		&m.Stage, &m.Group, &m.Matchday,
		&m.HadRedCard, &m.HadPenalty, &m.HadOwnGoal, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *matchRepository) GetUpcoming(ctx context.Context, from, to time.Time) ([]model.Match, error) {
	sql, args, err := psql.
		Select("id", "external_id", "home_team", "away_team", "match_date", "status", "home_score", "away_score", "stage", "group_name", "matchday", "updated_at").
		From("wc2026_matches").
		Where(sq.GtOrEq{"match_date": from.UTC()}).
		Where(sq.LtOrEq{"match_date": to.UTC()}).
		OrderBy("match_date ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []model.Match
	for rows.Next() {
		var m model.Match
		if err := rows.Scan(
			&m.ID, &m.ExternalID, &m.HomeTeam, &m.AwayTeam,
			&m.MatchDate, &m.Status, &m.HomeScore, &m.AwayScore,
			&m.Stage, &m.Group, &m.Matchday, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (r *matchRepository) GetFinishedInWindow(ctx context.Context, from, to time.Time) ([]model.Match, error) {
	sql, args, err := psql.
		Select("id", "external_id", "home_team", "away_team", "match_date", "status", "home_score", "away_score", "stage", "group_name", "matchday", "updated_at").
		From("wc2026_matches").
		Where(sq.GtOrEq{"match_date": from.UTC()}).
		Where(sq.Lt{"match_date": to.UTC()}).
		Where("status = ?", string(model.MatchStatusFinished)).
		OrderBy("match_date ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []model.Match
	for rows.Next() {
		var m model.Match
		if err := rows.Scan(
			&m.ID, &m.ExternalID, &m.HomeTeam, &m.AwayTeam,
			&m.MatchDate, &m.Status, &m.HomeScore, &m.AwayScore,
			&m.Stage, &m.Group, &m.Matchday, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (r *matchRepository) GetFinishedWithPendingScores(ctx context.Context) ([]model.Match, error) {
	sql, args, err := psql.
		Select("m.id", "m.external_id", "m.home_team", "m.away_team", "m.match_date",
			"m.status", "m.home_score", "m.away_score", "m.stage", "m.group_name", "m.matchday",
			"m.had_red_card", "m.had_penalty", "m.had_own_goal", "m.updated_at").
		Distinct().
		From("wc2026_matches m").
		Join("predictions p ON p.match_id = m.id").
		Where("m.status = ? AND p.points IS NULL", string(model.MatchStatusFinished)).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []model.Match
	for rows.Next() {
		var m model.Match
		if err := rows.Scan(
			&m.ID, &m.ExternalID, &m.HomeTeam, &m.AwayTeam,
			&m.MatchDate, &m.Status, &m.HomeScore, &m.AwayScore,
			&m.Stage, &m.Group, &m.Matchday,
			&m.HadRedCard, &m.HadPenalty, &m.HadOwnGoal, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// GetFinishedForEventSync returns FINISHED matches that have at least one risky-bet
// prediction and haven't had their event flags set yet (had_red_card IS NULL).
func (r *matchRepository) GetFinishedForEventSync(ctx context.Context) ([]model.Match, error) {
	sql, args, err := psql.
		Select("DISTINCT m.id", "m.external_id", "m.sofascore_id", "m.home_team", "m.away_team",
			"m.match_date", "m.status", "m.home_score", "m.away_score",
			"m.stage", "m.group_name", "m.matchday", "m.updated_at").
		From("wc2026_matches m").
		Join("predictions p ON p.match_id = m.id").
		Where("m.status = ? AND m.had_red_card IS NULL AND (p.bet_penalty OR p.bet_red_card OR p.bet_own_goal)",
			string(model.MatchStatusFinished)).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []model.Match
	for rows.Next() {
		var m model.Match
		if err := rows.Scan(
			&m.ID, &m.ExternalID, &m.SofascoreID, &m.HomeTeam, &m.AwayTeam,
			&m.MatchDate, &m.Status, &m.HomeScore, &m.AwayScore,
			&m.Stage, &m.Group, &m.Matchday, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (r *matchRepository) GetInPlay(ctx context.Context) ([]model.Match, error) {
	sql, args, err := psql.
		Select("id", "external_id", "home_team", "away_team", "match_date",
			"status", "home_score", "away_score", "stage", "group_name", "matchday",
			"had_red_card", "had_penalty", "had_own_goal", "updated_at").
		From("wc2026_matches").
		Where("status = ?", string(model.MatchStatusInPlay)).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []model.Match
	for rows.Next() {
		var m model.Match
		if err := rows.Scan(
			&m.ID, &m.ExternalID, &m.HomeTeam, &m.AwayTeam,
			&m.MatchDate, &m.Status, &m.HomeScore, &m.AwayScore,
			&m.Stage, &m.Group, &m.Matchday,
			&m.HadRedCard, &m.HadPenalty, &m.HadOwnGoal, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (r *matchRepository) UpdateEvents(ctx context.Context, matchID, sofascoreID int64, hadRedCard, hadPenalty, hadOwnGoal bool) error {
	sql, args, err := psql.
		Update("wc2026_matches").
		Set("sofascore_id", sofascoreID).
		Set("had_red_card", hadRedCard).
		Set("had_penalty", hadPenalty).
		Set("had_own_goal", hadOwnGoal).
		Set("updated_at", sq.Expr("NOW()")).
		Where("id = ?", matchID).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, sql, args...)
	return err
}

func (r *matchRepository) GetGroupStandings(ctx context.Context, groupName string) ([]model.StandingEntry, error) {
	// Collect all teams in the group, then left-join finished match results.
	query := `
WITH teams AS (
    SELECT home_team AS team FROM wc2026_matches WHERE group_name = $1
    UNION
    SELECT away_team             FROM wc2026_matches WHERE group_name = $1
),
results AS (
    SELECT home_team AS team,
           home_score AS gf, away_score AS ga,
           CASE WHEN home_score > away_score THEN 1 ELSE 0 END AS won,
           CASE WHEN home_score = away_score THEN 1 ELSE 0 END AS drawn,
           CASE WHEN home_score < away_score THEN 1 ELSE 0 END AS lost
    FROM wc2026_matches
    WHERE group_name = $1 AND status = 'FINISHED'
    UNION ALL
    SELECT away_team,
           away_score, home_score,
           CASE WHEN away_score > home_score THEN 1 ELSE 0 END,
           CASE WHEN away_score = home_score THEN 1 ELSE 0 END,
           CASE WHEN away_score < home_score THEN 1 ELSE 0 END
    FROM wc2026_matches
    WHERE group_name = $1 AND status = 'FINISHED'
)
SELECT t.team,
       COALESCE(SUM(r.won + r.drawn + r.lost), 0) AS played,
       COALESCE(SUM(r.won),   0) AS won,
       COALESCE(SUM(r.drawn), 0) AS drawn,
       COALESCE(SUM(r.lost),  0) AS lost,
       COALESCE(SUM(r.gf),    0) AS gf,
       COALESCE(SUM(r.ga),    0) AS ga,
       COALESCE(SUM(r.won * 3 + r.drawn), 0) AS points
FROM teams t
LEFT JOIN results r ON r.team = t.team
GROUP BY t.team
ORDER BY points DESC, (COALESCE(SUM(r.gf),0) - COALESCE(SUM(r.ga),0)) DESC, COALESCE(SUM(r.gf),0) DESC, t.team ASC`

	rows, err := r.db.Query(ctx, query, groupName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.StandingEntry
	for rows.Next() {
		var e model.StandingEntry
		if err := rows.Scan(&e.Team, &e.Played, &e.Won, &e.Drawn, &e.Lost, &e.GF, &e.GA, &e.Points); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
