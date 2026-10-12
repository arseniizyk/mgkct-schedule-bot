package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type TeacherScheduleRepository interface {
	Save(ctx context.Context, name string, week time.Time, schedule *entities.Teacher) error
	GetByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	GetLatest(ctx context.Context, name string) (*entities.Teacher, error)
	GetAllLatest(ctx context.Context) (map[string]*entities.Teacher, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
}

type TeacherService struct {
	log  *slog.Logger
	repo TeacherScheduleRepository
}

func NewTeacherService(log *slog.Logger, repo TeacherScheduleRepository) *TeacherService {
	return &TeacherService{
		log:  log,
		repo: repo,
	}
}

func (s *TeacherService) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	const op = "service.TeacherService.GetTeacherLatestSchedule"
	log := s.log.With("operation", op, "name", name)

	t, err := s.repo.GetLatest(ctx, name)
	if err != nil {
		if errors.Is(err, domainerr.ErrScheduleNotFound) {
			log.WarnContext(ctx, "teacher schedule not found in repository")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by latest", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return t, nil
}

func (s *TeacherService) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	const op = "service.TeacherService.GetTeacherScheduleByWeek"
	log := s.log.With("operation", op, "name", name, "week", week)

	t, err := s.repo.GetByWeek(ctx, name, week)
	if err != nil {
		if errors.Is(err, domainerr.ErrWeekNotFound) {
			log.WarnContext(ctx, "teacher week not found")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by week", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return t, nil
}

func (s *TeacherService) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return s.repo.GetAllTeacherNames(ctx)
}

func (s *TeacherService) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return s.repo.GetWeeks(ctx, name, week)
}

func (s *TeacherService) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	exact, partial, err := s.FindTeacherByName(ctx, name)
	if err != nil {
		s.log.ErrorContext(ctx, "failed finding teacher by name", "name", name, "error", err)
		return "", nil, false
	}

	// Если есть ровно одно точное совпадение — отдаем его сразу
	if len(exact) == 1 {
		return exact[0], nil, true
	}

	// Если точных нет, проверяем частичные совпадения
	if len(exact) == 0 {
		switch len(partial) {
		case 0:
			return "", nil, false
		case 1:
			return partial[0], nil, true
		default:
			return "", partial, false
		}
	}

	// Если почему-то точных совпадений > 1
	return "", exact, false
}

func (s *TeacherService) FindTeacherByName(ctx context.Context, name string) (exact, partial []string, err error) {
	names, err := s.repo.GetAllTeacherNames(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get all teacher names: %w", err)
	}

	lowerName := strings.ToLower(strings.TrimSpace(name))

	for _, n := range names {
		lower := strings.ToLower(n)
		switch {
		case lower == lowerName:
			exact = append(exact, n)
		case strings.Contains(lower, lowerName):
			partial = append(partial, n)
		}
	}

	return exact, partial, nil
}
