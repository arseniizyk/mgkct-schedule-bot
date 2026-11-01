package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type FacultyRepository struct {
	db *sql.DB
	sb squirrel.StatementBuilderType
}

func NewFacultyRepository(db *sql.DB) *FacultyRepository {
	return &FacultyRepository{
		db: db,
		sb: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question),
	}
}

// SaveBatch сохраняет или обновляет список преподавателей и сотрудников
func (r *FacultyRepository) SaveBatch(ctx context.Context, members []*entities.FacultyMember) error {
	const op = "repository.FacultyRepository.SaveBatch"

	if len(members) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertQuery = `
		INSERT INTO faculty (
			full_name, short_name, position, qualification, education, experience, department, full_name_lower, short_name_lower
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(full_name, department) DO UPDATE SET
			short_name = excluded.short_name,
			position = excluded.position,
			qualification = excluded.qualification,
			education = excluded.education,
			experience = excluded.experience,
			full_name_lower = excluded.full_name_lower,
			short_name_lower = excluded.short_name_lower,
			updated_at = CURRENT_TIMESTAMP
	`

	stmt, err := tx.PrepareContext(ctx, insertQuery)
	if err != nil {
		return fmt.Errorf("%s: prepare stmt: %w", op, err)
	}
	defer stmt.Close()

	for _, m := range members {
		fullNameLower := strings.ToLower(m.FullName)
		shortNameLower := strings.ToLower(m.ShortName)

		if _, err := stmt.ExecContext(
			ctx,
			m.FullName,
			m.ShortName,
			m.Position,
			m.Qualification,
			m.Education,
			m.Experience,
			m.Department,
			fullNameLower,
			shortNameLower,
		); err != nil {
			return fmt.Errorf("%s: exec sql: %w", op, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit tx: %w", op, err)
	}

	return nil
}

// GetAll возвращает всех сохраненных сотрудников
func (r *FacultyRepository) GetAll(ctx context.Context) ([]*entities.FacultyMember, error) {
	const op = "repository.FacultyRepository.GetAll"

	query := r.sb.Select("id", "full_name", "short_name", "position", "qualification", "education", "experience", "department", "updated_at").
		From("faculty").
		OrderBy("full_name ASC")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build sql: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query faculty: %w", op, err)
	}
	defer rows.Close()

	var list []*entities.FacultyMember
	for rows.Next() {
		m := &entities.FacultyMember{}
		if err := rows.Scan(
			&m.ID,
			&m.FullName,
			&m.ShortName,
			&m.Position,
			&m.Qualification,
			&m.Education,
			&m.Experience,
			&m.Department,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", op, err)
		}
		list = append(list, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return list, nil
}

// FindByShortName ищет преподавателя по сокращенному имени (например, "Смирнов С. С.")
func (r *FacultyRepository) FindByShortName(ctx context.Context, shortName string) (*entities.FacultyMember, error) {
	const op = "repository.FacultyRepository.FindByShortName"

	query := r.sb.Select("id", "full_name", "short_name", "position", "qualification", "education", "experience", "department", "updated_at").
		From("faculty").
		Where(squirrel.Eq{"short_name": strings.TrimSpace(shortName)}).
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build sql: %w", op, err)
	}

	row := r.db.QueryRowContext(ctx, sqlQuery, args...)
	m := &entities.FacultyMember{}
	if err := row.Scan(
		&m.ID,
		&m.FullName,
		&m.ShortName,
		&m.Position,
		&m.Qualification,
		&m.Education,
		&m.Experience,
		&m.Department,
		&m.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerr.ErrTeacherNotFound
		}
		return nil, fmt.Errorf("%s: scan row: %w", op, err)
	}

	return m, nil
}

// Search ищет сотрудников по частичному совпадению ФИО
func (r *FacultyRepository) Search(ctx context.Context, queryStr string) ([]*entities.FacultyMember, error) {
	const op = "repository.FacultyRepository.Search"

	pattern := "%" + strings.ToLower(strings.TrimSpace(queryStr)) + "%"
	query := r.sb.Select("id", "full_name", "short_name", "position", "qualification", "education", "experience", "department", "updated_at").
		From("faculty").
		Where(squirrel.Or{
			squirrel.Expr("full_name_lower LIKE ?", pattern),
			squirrel.Expr("short_name_lower LIKE ?", pattern),
			squirrel.Expr("LOWER(full_name) LIKE ?", pattern),
			squirrel.Expr("LOWER(short_name) LIKE ?", pattern),
		}).
		OrderBy("full_name ASC")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build sql: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query faculty: %w", op, err)
	}
	defer rows.Close()

	var list []*entities.FacultyMember
	for rows.Next() {
		m := &entities.FacultyMember{}
		if err := rows.Scan(
			&m.ID,
			&m.FullName,
			&m.ShortName,
			&m.Position,
			&m.Qualification,
			&m.Education,
			&m.Experience,
			&m.Department,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", op, err)
		}
		list = append(list, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return list, nil
}
