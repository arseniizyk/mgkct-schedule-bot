package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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
	GetAll(ctx context.Context) ([]*entities.Teacher, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	GetAllWeeks(ctx context.Context, name string) ([]time.Time, error)
	GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
}

type FacultyRepository interface {
	SaveBatch(ctx context.Context, members []*entities.FacultyMember) error
	GetAll(ctx context.Context) ([]*entities.FacultyMember, error)
	FindByShortName(ctx context.Context, shortName string) (*entities.FacultyMember, error)
	Search(ctx context.Context, queryStr string) ([]*entities.FacultyMember, error)
}

type SubjectDictionary interface {
	Lookup(shortName string) (string, bool)
	Resolve(raw string) (cleaned string, fullName string, found bool)
	All() map[string]string
	Search(query string) []entities.SubjectInfo
}

type TeacherService struct {
	log         *slog.Logger
	repo        TeacherScheduleRepository
	facultyRepo FacultyRepository
	dict        SubjectDictionary
}

func NewTeacherService(
	log *slog.Logger,
	repo TeacherScheduleRepository,
	facultyRepo FacultyRepository,
	dict SubjectDictionary,
) *TeacherService {
	return &TeacherService{
		log:         log,
		repo:        repo,
		facultyRepo: facultyRepo,
		dict:        dict,
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

	// 1. Прямой поиск по расписанию преподавателей
	for _, n := range names {
		lower := strings.ToLower(n)
		switch {
		case lower == lowerName:
			exact = append(exact, n)
		case strings.Contains(lower, lowerName):
			partial = append(partial, n)
		}
	}

	// 2. Если точных нет и подключен репозиторий педсостава, ищем по полному ФИО в справочнике
	if len(exact) == 0 && s.facultyRepo != nil {
		members, err := s.facultyRepo.Search(ctx, name)
		if err == nil {
			namesSet := make(map[string]struct{}, len(names))
			for _, n := range names {
				namesSet[n] = struct{}{}
			}

			for _, m := range members {
				if _, ok := namesSet[m.ShortName]; !ok {
					continue
				}
				if strings.EqualFold(m.FullName, strings.TrimSpace(name)) {
					if !slices.Contains(exact, m.ShortName) {
						exact = append(exact, m.ShortName)
					}
				} else {
					if !slices.Contains(partial, m.ShortName) && !slices.Contains(exact, m.ShortName) {
						partial = append(partial, m.ShortName)
					}
				}
			}
		}
	}

	return exact, partial, nil
}

func (s *TeacherService) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	if s.facultyRepo == nil {
		return nil, domainerr.ErrTeacherNotFound
	}
	return s.facultyRepo.FindByShortName(ctx, teacherName)
}

func (s *TeacherService) GetAllFaculty(ctx context.Context) ([]*entities.FacultyMember, error) {
	if s.facultyRepo == nil {
		return nil, domainerr.ErrTeacherNotFound
	}
	return s.facultyRepo.GetAll(ctx)
}

func (s *TeacherService) GetSubjectDictionary() SubjectDictionary {
	return s.dict
}

func (s *TeacherService) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	return s.repo.GetAllWeeks(ctx, name)
}

func (s *TeacherService) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	const op = "service.TeacherService.GetTeacherArchiveSchedule"

	targetDate = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.UTC)
	weekday := (int(targetDate.Weekday()) + 6) % 7 // Пн=0..Вс=6
	monday := targetDate.AddDate(0, 0, -weekday)

	t, err := s.GetTeacherScheduleByWeek(ctx, name, monday)
	if err == nil {
		return t, nil
	}

	t, err = s.repo.GetByWeek(ctx, name, targetDate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	diff := targetDate.Sub(t.Week)
	if diff < 0 || diff > 7*24*time.Hour {
		return nil, domainerr.ErrWeekNotFound
	}

	return t, nil
}
