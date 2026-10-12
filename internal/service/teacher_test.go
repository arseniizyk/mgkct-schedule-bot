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
	_ = tRepo.Save(ctx, "Анципов Е. Ю.", time.Now(), &entities.Teacher{Name: "Анципов Е. Ю."})
	_ = tRepo.Save(ctx, "Иванов И. И.", time.Now(), &entities.Teacher{Name: "Иванов И. И."})

	fRepo := &mockFacultyRepo{
		members: []*entities.FacultyMember{
			{
				FullName:  "Анципов Евгений Юрьевич",
				ShortName: "Анципов Е. Ю.",
				Position:  "Преподаватель",
			},
		},
	}

	svc := service.NewTeacherService(slog.Default(), tRepo, fRepo, nil)

	// 1. Поиск по точному краткому имени
	matched, candidates, ok := svc.ValidateTeacher(ctx, "Анципов Е. Ю.")
	assert.True(t, ok)
	assert.Equal(t, "Анципов Е. Ю.", matched)
	assert.Empty(t, candidates)

	// 2. Поиск по полному ФИО (через справочник педсостава)
	matched, candidates, ok = svc.ValidateTeacher(ctx, "Анципов Евгений Юрьевич")
	assert.True(t, ok)
	assert.Equal(t, "Анципов Е. Ю.", matched)
	assert.Empty(t, candidates)

	// 3. Поиск по фамилии
	matched, candidates, ok = svc.ValidateTeacher(ctx, "Анципов")
	assert.True(t, ok)
	assert.Equal(t, "Анципов Е. Ю.", matched)
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
