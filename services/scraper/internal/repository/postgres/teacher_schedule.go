package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
)

type TeacherScheduleRepository struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

func NewTeacherScheduleRepository(pool *pgxpool.Pool) *TeacherScheduleRepository {
	return &TeacherScheduleRepository{
		pool: pool,
		sb:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (repo *TeacherScheduleRepository) Save(ctx context.Context, name string, week time.Time, schedule *pb.Teacher) error {
	const op = "repository.postgres.TeacherScheduleRepository.Save"

	data, err := protojson.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("%s: marshal teacher schedule: %w", op, err)
	}

	query := repo.sb.Insert("teacher_schedules").
		Columns("name", "week", "schedule").
		Values(name, week, data).
		Suffix("ON CONFLICT (name, week) DO UPDATE SET schedule = EXCLUDED.schedule")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	if _, err := repo.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s: exec sql query: %w", op, err)
	}

	return nil
}

func (repo *TeacherScheduleRepository) GetByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error) {
	const op = "repository.postgres.TeacherScheduleRepository.GetByWeek"

	query := repo.sb.Select("schedule").
		From("teacher_schedules").
		Where(squirrel.Eq{"name": name}).
		Where(squirrel.LtOrEq{"week": week}).
		OrderBy("week DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw []byte
	if err := repo.pool.QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrWeekNotFound
		}
		return nil, fmt.Errorf("%s: get teacher schedule: %w", op, err)
	}

	var t pb.Teacher
	if err := protojson.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("%s: unmarshal teacher schedule: %w", op, err)
	}

	return &t, nil
}

func (repo *TeacherScheduleRepository) GetLatest(ctx context.Context, name string) (*pb.Teacher, error) {
	const op = "repository.postgres.TeacherScheduleRepository.GetLatest"

	query := repo.sb.Select("schedule").
		From("teacher_schedules").
		Where(squirrel.Eq{"name": name}).
		OrderBy("updated_at DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw []byte
	err = repo.pool.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrScheduleNotFound
		}
		return nil, fmt.Errorf("%s: get latest teacher schedule: %w", op, err)
	}

	var t pb.Teacher
	if err := protojson.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("%s: unmarshal teacher schedule: %w", op, err)
	}

	return &t, nil
}

func (repo *TeacherScheduleRepository) GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	const op = "repository.postgres.TeacherScheduleRepository.GetWeeks"

	week = time.Date(week.Year(), week.Month(), week.Day(), 0, 0, 0, 0, time.UTC)

	if week.IsZero() {
		query := repo.sb.Select("week").
			From("teacher_schedules").
			OrderBy("week DESC").
			Limit(2)

		sql, args, err := query.ToSql()
		if err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: failed to create query: %w", op, err)
		}

		rows, err := repo.pool.Query(ctx, sql, args...)
		if err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: repository sql query: %w", op, err)
		}
		defer rows.Close()

		var weeks []time.Time
		for rows.Next() {
			var w time.Time
			if err := rows.Scan(&w); err != nil {
				return entities.WeekNavigation{}, fmt.Errorf("%s: repository scan rows: %w", op, err)
			}
			weeks = append(weeks, w)
		}
		if err := rows.Err(); err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: sql rows err: %w", op, err)
		}

		if len(weeks) < 2 {
			return entities.WeekNavigation{}, repository.ErrNoAvailableWeeks
		}

		return entities.WeekNavigation{
			Current: weeks[0],
			Prev:    weeks[1],
		}, nil
	}

	var current, prev, next time.Time

	if err := repo.getWeek(ctx, "DESC", squirrel.Eq{"week": week}, &current); err != nil {
		return entities.WeekNavigation{}, fmt.Errorf("%s: get current week: %w", op, err)
	}

	if err := repo.getWeek(ctx, "DESC", squirrel.Lt{"week": week}, &prev); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return entities.WeekNavigation{}, fmt.Errorf("%s: get prev week: %w", op, err)
		}
		prev = time.Time{}
	}

	if err := repo.getWeek(ctx, "ASC", squirrel.Gt{"week": week}, &next); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return entities.WeekNavigation{}, fmt.Errorf("%s: get next week: %w", op, err)
		}
		next = time.Time{}
	}

	return entities.WeekNavigation{
		Prev:    prev,
		Current: current,
		Next:    next,
	}, nil
}

func (repo *TeacherScheduleRepository) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	const op = "repository.postgres.TeacherScheduleRepository.GetAllTeacherNames"

	query := repo.sb.Select("DISTINCT name").
		From("teacher_schedules").
		OrderBy("name ASC")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	rows, err := repo.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query teacher names: %w", op, err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("%s: scan teacher name: %w", op, err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return names, nil
}

func (repo *TeacherScheduleRepository) GetAllLatest(ctx context.Context) (map[string]*pb.Teacher, error) {
	const op = "repository.postgres.TeacherScheduleRepository.GetAllLatest"

	query, args, err := repo.sb.
		Select("DISTINCT ON (ts.name) ts.name", "ts.schedule").
		From("teacher_schedules ts").
		OrderBy("ts.name", "ts.updated_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}

	rows, err := repo.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query all latest: %w", op, err)
	}
	defer rows.Close()

	result := make(map[string]*pb.Teacher)
	for rows.Next() {
		var name string
		var raw []byte
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", op, err)
		}

		var t pb.Teacher
		if err := protojson.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("%s: unmarshal teacher: %w", op, err)
		}

		result[name] = &t
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return result, nil
}

func (repo *TeacherScheduleRepository) getWeek(ctx context.Context, orderBy string, pred, dest any) error {
	query := repo.sb.Select("week").
		From("teacher_schedules").
		Where(pred).
		OrderBy("week " + orderBy).
		Limit(1)
	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	row := repo.pool.QueryRow(ctx, sql, args...)
	if err := row.Scan(dest); err != nil {
		return err
	}

	return nil
}
