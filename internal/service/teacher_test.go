package service_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type mockFacultyRepo struct {
	members []*entities.FacultyMember
}

func (m *mockFacultyRepo) SaveBatch(ctx context.Context, members []*entities.FacultyMember) error {
	m.members = append(m.members, members...)
	return nil
}

func (m *mockFacultyRepo) GetAll(ctx context.Context) ([]*entities.FacultyMember, error) {
	return m.members, nil
}

func (m *mockFacultyRepo) FindByShortName(ctx context.Context, shortName string) (*entities.FacultyMember, error) {
	for _, f := range m.members {
		if f.ShortName == shortName {
			return f, nil
		}
	}
	return nil, nil
}

func (m *mockFacultyRepo) Search(ctx context.Context, queryStr string) ([]*entities.FacultyMember, error) {
	var res []*entities.FacultyMember
	q := strings.ToLower(strings.TrimSpace(queryStr))
	for _, f := range m.members {
		if strings.Contains(strings.ToLower(f.FullName), q) || strings.Contains(strings.ToLower(f.ShortName), q) {
			res = append(res, f)
		}
	}
	return res, nil
}

func TestTeacherService_ValidateTeacher_WithFaculty(t *testing.T) {
	ctx := context.Background()

	tRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}
	_ = tRepo.Save(ctx, "Смирнов С. С.", time.Now(), &entities.Teacher{Name: "Смирнов С. С."})
	_ = tRepo.Save(ctx, "Иванов И. И.", time.Now(), &entities.Teacher{Name: "Иванов И. И."})

	fRepo := &mockFacultyRepo{
		members: []*entities.FacultyMember{
			{
				FullName:  "Смирнов Сергей Сергеевич",
				ShortName: "Смирнов С. С.",
				Position:  "Преподаватель",
			},
		},
	}

	svc := service.NewTeacherService(slog.Default(), tRepo, fRepo, nil)

	// 1. Поиск по точному краткому имени
	matched, candidates, ok := svc.ValidateTeacher(ctx, "Смирнов С. С.")
	assert.True(t, ok)
	assert.Equal(t, "Смирнов С. С.", matched)
	assert.Empty(t, candidates)

	// 2. Поиск по полному ФИО (через справочник педсостава)
	matched, candidates, ok = svc.ValidateTeacher(ctx, "Смирнов Сергей Сергеевич")
	assert.True(t, ok)
	assert.Equal(t, "Смирнов С. С.", matched)
	assert.Empty(t, candidates)

	// 3. Поиск по фамилии
	matched, candidates, ok = svc.ValidateTeacher(ctx, "Смирнов")
	assert.True(t, ok)
	assert.Equal(t, "Смирнов С. С.", matched)
	assert.Empty(t, candidates)

	// 4. Несуществующий преподаватель
	matched, candidates, ok = svc.ValidateTeacher(ctx, "Неизвестный")
	assert.False(t, ok)
	assert.Empty(t, matched)
	assert.Empty(t, candidates)
}

func TestTeacherService_Archive(t *testing.T) {
	ctx := context.Background()

	week, _ := time.Parse("2006-01-02", "2026-10-12")
	tRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}
	_ = tRepo.Save(ctx, "Иванов И. И.", week, &entities.Teacher{Name: "Иванов И. И.", Week: week})

	svc := service.NewTeacherService(slog.Default(), tRepo, nil, nil)

	// GetAllWeeks
	weeks, err := svc.GetAllWeeks(ctx, "Иванов И. И.")
	require.NoError(t, err)
	assert.Len(t, weeks, 1)

	// GetTeacherArchiveSchedule by exact monday
	targetMonday, _ := time.Parse("2006-01-02", "2026-10-12")
	teach, err := svc.GetTeacherArchiveSchedule(ctx, "Иванов И. И.", targetMonday)
	require.NoError(t, err)
	assert.Equal(t, "Иванов И. И.", teach.Name)

	// GetTeacherArchiveSchedule by midweek date (Wednesday 14.10)
	midweek, _ := time.Parse("2006-01-02", "2026-10-14")
	teach, err = svc.GetTeacherArchiveSchedule(ctx, "Иванов И. И.", midweek)
	require.NoError(t, err)
	assert.Equal(t, "Иванов И. И.", teach.Name)
}

type strictExactWeekTeacherRepo struct {
	mockTeacherRepo
}

func (s *strictExactWeekTeacherRepo) GetByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	if s.teachers[name] == nil {
		return nil, domainerr.ErrWeekNotFound
	}
	t, ok := s.teachers[name][week]
	if !ok {
		return nil, domainerr.ErrWeekNotFound
	}
	return t, nil
}

func TestTeacherService_GetTeacherArchiveSchedule_MidweekStrictRepo(t *testing.T) {
	ctx := context.Background()
	monday, _ := time.Parse("2006-01-02", "2026-10-12")
	midweek, _ := time.Parse("2006-01-02", "2026-10-14")

	repo := &strictExactWeekTeacherRepo{
		mockTeacherRepo{
			teachers: map[string]map[time.Time]*entities.Teacher{
				"Иванов И. И.": {
					monday: {Name: "Иванов И. И.", Week: monday},
				},
			},
		},
	}

	svc := service.NewTeacherService(slog.Default(), repo, nil, nil)
	teach, err := svc.GetTeacherArchiveSchedule(ctx, "Иванов И. И.", midweek)
	require.NoError(t, err)
	assert.Equal(t, "Иванов И. И.", teach.Name)

	// Несуществующий преподаватель
	_, err = svc.GetTeacherArchiveSchedule(ctx, "Петров П. П.", midweek)
	require.Error(t, err)

	// Несуществующая неделя
	otherWeek, _ := time.Parse("2006-01-02", "2026-10-21")
	_, err = svc.GetTeacherArchiveSchedule(ctx, "Иванов И. И.", otherWeek)
	require.Error(t, err)
}

func TestTeacherService_FindSimilarTeachers(t *testing.T) {
	ctx := context.Background()

	tRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}
	now := time.Now()
	_ = tRepo.Save(ctx, "Смирнов С. С.", now, &entities.Teacher{Name: "Смирнов С. С."})
	_ = tRepo.Save(ctx, "Иванов И. И.", now, &entities.Teacher{Name: "Иванов И. И."})
	_ = tRepo.Save(ctx, "Иванова А. А.", now, &entities.Teacher{Name: "Иванова А. А."})
	_ = tRepo.Save(ctx, "Новиков П. С.", now, &entities.Teacher{Name: "Новиков П. С."})
	_ = tRepo.Save(ctx, "Морозов Д. В.", now, &entities.Teacher{Name: "Морозов Д. В."})
	_ = tRepo.Save(ctx, "Волков И. И.", now, &entities.Teacher{Name: "Волков И. И."})
	_ = tRepo.Save(ctx, "Фёдоров Ф. Ф.", now, &entities.Teacher{Name: "Фёдоров Ф. Ф."})

	fRepo := &mockFacultyRepo{
		members: []*entities.FacultyMember{
			{
				FullName:  "Смирнов Сергей Сергеевич",
				ShortName: "Смирнов С. С.",
				Position:  "Преподаватель",
			},
		},
	}

	svc := service.NewTeacherService(slog.Default(), tRepo, fRepo, nil)

	t.Run("typo in one letter", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Смирнав", 3)
		require.NoError(t, err)
		require.NotEmpty(t, suggestions)
		assert.Equal(t, "Смирнов С. С.", suggestions[0])
	})

	t.Run("typo in two letters", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Смырнав", 3)
		require.NoError(t, err)
		require.NotEmpty(t, suggestions)
		assert.Equal(t, "Смирнов С. С.", suggestions[0])
	})

	t.Run("transposition typo", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Смринов", 3)
		require.NoError(t, err)
		require.NotEmpty(t, suggestions)
		assert.Equal(t, "Смирнов С. С.", suggestions[0])
	})

	t.Run("letter yo vs e", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Федоров", 3)
		require.NoError(t, err)
		require.NotEmpty(t, suggestions)
		assert.Equal(t, "Фёдоров Ф. Ф.", suggestions[0])
	})

	t.Run("keyboard layout typo", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "cvbhyjd", 3)
		require.NoError(t, err)
		require.NotEmpty(t, suggestions)
		assert.Equal(t, "Смирнов С. С.", suggestions[0])
	})

	t.Run("typo with initials", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Смирнав С. С.", 3)
		require.NoError(t, err)
		require.NotEmpty(t, suggestions)
		assert.Equal(t, "Смирнов С. С.", suggestions[0])
	})

	t.Run("limit to 3 suggestions", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Иваноф", 3)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(suggestions), 3)
		assert.NotEmpty(t, suggestions)
	})

	t.Run("completely unrelated word returns empty", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Абракадабра", 3)
		require.NoError(t, err)
		assert.Empty(t, suggestions)
	})

	t.Run("short query returns empty", func(t *testing.T) {
		suggestions, err := svc.FindSimilarTeachers(ctx, "Ан", 3)
		require.NoError(t, err)
		assert.Empty(t, suggestions)
	})
}

func TestTeacherService_WeeksCache_TTL_And_Invalidation(t *testing.T) {
	ctx := context.Background()
	week, _ := time.Parse("2006-01-02", "2026-10-12")

	tRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}
	_ = tRepo.Save(ctx, "Иванов И. И.", week, &entities.Teacher{
		Name: "Иванов И. И.",
		Week: week,
		Days: []*entities.Day{},
	})

	svc := service.NewTeacherService(slog.Default(), tRepo, nil, nil)

	// 1. Читаем — попадает в кэш
	t1, err := svc.GetTeacherScheduleByWeek(ctx, "Иванов И. И.", week)
	require.NoError(t, err)
	assert.Equal(t, "Иванов И. И.", t1.Name)

	// 2. Модифицируем в репозитории
	_ = tRepo.Save(ctx, "Иванов И. И.", week, &entities.Teacher{
		Name: "Иванов И. И.",
		Week: week,
		Days: []*entities.Day{{Name: "Понедельник"}},
	})

	// 3. Должна возвращаться закэшированная версия
	cachedT, err := svc.GetTeacherScheduleByWeek(ctx, "Иванов И. И.", week)
	require.NoError(t, err)
	assert.Empty(t, cachedT.Days, "должна возвращаться закэшированная версия")

	// 4. Инвалидируем конкретного преподавателя
	svc.InvalidateTeacherWeek("Иванов И. И.", week)

	// 5. Теперь возвращается свежая версия
	freshT, err := svc.GetTeacherScheduleByWeek(ctx, "Иванов И. И.", week)
	require.NoError(t, err)
	assert.Len(t, freshT.Days, 1, "после инвалидации должна возвращаться свежая версия")

	// 6. Проверяем ClearWeeksCache
	svc.ClearWeeksCache()
	freshT2, err := svc.GetTeacherScheduleByWeek(ctx, "Иванов И. И.", week)
	require.NoError(t, err)
	assert.Len(t, freshT2.Days, 1)
}
