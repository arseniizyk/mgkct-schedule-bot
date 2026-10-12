package service_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
