package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

const (
	digestKindGroup   = "group"
	digestKindTeacher = "teacher"
)

// DigestSentRepository хранит информацию о том, каким группам/преподавателям
// уже был отправлен вечерний дайджест сегодня.
// Это позволяет корректно возобновить работу после перезапуска бота —
// без повторной рассылки пользователям, уже получившим уведомление.
type DigestSentRepository struct {
	db *sql.DB
	sb sq.StatementBuilderType
}

// NewDigestSentRepository создаёт новый DigestSentRepository.
func NewDigestSentRepository(db *sql.DB) *DigestSentRepository {
	return &DigestSentRepository{
		db: db,
		sb: sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}
}

// IsGroupDigestSent возвращает true, если дайджест для группы/подгруппы уже был отправлен сегодня.
func (r *DigestSentRepository) IsGroupDigestSent(ctx context.Context, groupID, subgroup int, date string) (bool, error) {
	const op = "repository.DigestSentRepository.IsGroupDigestSent"
	key := fmt.Sprintf("%d:%d", groupID, subgroup)
	return r.isSent(ctx, op, digestKindGroup, key, date)
}

// MarkGroupDigestSent отмечает, что дайджест для группы/подгруппы был отправлен сегодня.
func (r *DigestSentRepository) MarkGroupDigestSent(ctx context.Context, groupID, subgroup int, date string) error {
	const op = "repository.DigestSentRepository.MarkGroupDigestSent"
	key := fmt.Sprintf("%d:%d", groupID, subgroup)
	return r.markSent(ctx, op, digestKindGroup, key, date)
}

// IsTeacherDigestSent возвращает true, если дайджест для преподавателя уже был отправлен сегодня.
func (r *DigestSentRepository) IsTeacherDigestSent(ctx context.Context, teacherName, date string) (bool, error) {
	const op = "repository.DigestSentRepository.IsTeacherDigestSent"
	return r.isSent(ctx, op, digestKindTeacher, teacherName, date)
}

// MarkTeacherDigestSent отмечает, что дайджест для преподавателя был отправлен сегодня.
func (r *DigestSentRepository) MarkTeacherDigestSent(ctx context.Context, teacherName, date string) error {
	const op = "repository.DigestSentRepository.MarkTeacherDigestSent"
	return r.markSent(ctx, op, digestKindTeacher, teacherName, date)
}

// DeleteOldEntries удаляет записи старше указанной даты.
// Рекомендуется вызывать раз в сутки для предотвращения роста таблицы.
func (r *DigestSentRepository) DeleteOldEntries(ctx context.Context, beforeDate string) error {
	const op = "repository.DigestSentRepository.DeleteOldEntries"

	query := r.sb.Delete("digest_sent_log").Where(sq.Lt{"sent_date": beforeDate})
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: exec query: %w", op, err)
	}

	return nil
}

func (r *DigestSentRepository) isSent(ctx context.Context, op, kind, key, date string) (bool, error) {
	query := r.sb.Select("1").From("digest_sent_log").
		Where(sq.Eq{"kind": kind, "target_key": key, "sent_date": date}).
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return false, fmt.Errorf("%s: build query: %w", op, err)
	}

	var dummy int
	err = r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&dummy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("%s: query row: %w", op, err)
	}

	return true, nil
}

func (r *DigestSentRepository) markSent(ctx context.Context, op, kind, key, date string) error {
	query := r.sb.Insert("digest_sent_log").
		Columns("kind", "target_key", "sent_date").
		Values(kind, key, date).
		Suffix("ON CONFLICT(kind, target_key, sent_date) DO NOTHING")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: exec query: %w", op, err)
	}

	return nil
}
