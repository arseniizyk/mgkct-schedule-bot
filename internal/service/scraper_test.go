package service_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type mockScheduleRepo struct {
	schedules map[time.Time]*entities.Schedule
}

func (m *mockScheduleRepo) Save(ctx context.Context, week time.Time, schedule *entities.Schedule) error {
	m.schedules[week] = schedule
	return nil
}

func (m *mockScheduleRepo) GetByWeek(ctx context.Context, week time.Time) (*entities.Schedule, error) {
	return m.schedules[week], nil
}

func (m *mockScheduleRepo) GetLatest(ctx context.Context) (*entities.Schedule, error) {
	var maxWeek time.Time
	for w := range m.schedules {
		if w.After(maxWeek) {
			maxWeek = w
		}
	}
	return m.schedules[maxWeek], nil
}

func (m *mockScheduleRepo) GetAll(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	return m.schedules, nil
}

func (m *mockScheduleRepo) GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

type mockTeacherRepo struct {
	teachers map[string]map[time.Time]*entities.Teacher
}

func (m *mockTeacherRepo) Save(ctx context.Context, name string, week time.Time, schedule *entities.Teacher) error {
	if m.teachers[name] == nil {
		m.teachers[name] = make(map[time.Time]*entities.Teacher)
	}
	m.teachers[name][week] = schedule
	return nil
}

func (m *mockTeacherRepo) GetByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	return m.teachers[name][week], nil
}

func (m *mockTeacherRepo) GetLatest(ctx context.Context, name string) (*entities.Teacher, error) {
	var maxWeek time.Time
	for w := range m.teachers[name] {
		if w.After(maxWeek) {
			maxWeek = w
		}
	}
	return m.teachers[name][maxWeek], nil
}

func (m *mockTeacherRepo) GetAllLatest(ctx context.Context) (map[string]*entities.Teacher, error) {
	res := make(map[string]*entities.Teacher)
	for name := range m.teachers {
		t, _ := m.GetLatest(ctx, name)
		res[name] = t
	}
	return res, nil
}

func (m *mockTeacherRepo) GetAll(ctx context.Context) ([]*entities.Teacher, error) {
	var res []*entities.Teacher
	for _, weeks := range m.teachers {
		for _, t := range weeks {
			res = append(res, t)
		}
	}
	return res, nil
}

func (m *mockTeacherRepo) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	var names []string
	for n := range m.teachers {
		names = append(names, n)
	}
	return names, nil
}

func (m *mockTeacherRepo) GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

type mockStudentParser struct {
	res map[time.Time]*entities.Schedule
}

func (p *mockStudentParser) Parse(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	return p.res, nil
}

type mockTeacherParser struct {
	res []*entities.Teacher
}

func (p *mockTeacherParser) Parse(ctx context.Context) ([]*entities.Teacher, error) {
	return p.res, nil
}

func TestScraperService_NoSpamWhenUnchanged(t *testing.T) {
	ctx := t.Context()

	week1 := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	week2 := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	t1w1 := &entities.Teacher{Name: "Иванов И. И.", Week: week1, Days: []*entities.Day{{Name: "Понедельник"}}}
	t1w2 := &entities.Teacher{Name: "Иванов И. И.", Week: week2, Days: []*entities.Day{{Name: "Вторник"}}}

	schedRepo := &mockScheduleRepo{schedules: make(map[time.Time]*entities.Schedule)}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	// Сохраняем в репозиторий преподавателя на 2 недели
	_ = teachRepo.Save(ctx, t1w1.Name, t1w1.Week, t1w1)
	_ = teachRepo.Save(ctx, t1w2.Name, t1w2.Week, t1w2)

	studentParser := &mockStudentParser{res: make(map[time.Time]*entities.Schedule)}
	teacherParser := &mockTeacherParser{res: []*entities.Teacher{t1w1, t1w2}}

	schedSvc := service.NewScheduleService(slog.Default(), schedRepo)
	scraperSvc := service.NewScraperService(
		slog.Default(),
		config.ScraperConfig{Interval: time.Hour, PeakInterval: time.Hour, NightInterval: time.Hour},
		studentParser,
		teacherParser,
		schedRepo,
		teachRepo,
		schedSvc,
	)

	eventCh := make(chan service.Event, 100)

	// Запускаем скрапер в горутине на короткое время
	go scraperSvc.Start(ctx, eventCh)

	// Ждем немного для отработки инициализации и первого тика
	time.Sleep(100 * time.Millisecond)

	// Событий быть НЕ должно, так как расписание не изменилось!
	assert.Empty(t, eventCh, "Скрапер не должен слать спам-события при неизменном расписании")
}

func TestScraperService_EmitsEventOnActualChange(t *testing.T) {
	ctx := t.Context()

	week1 := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	t1w1 := &entities.Teacher{Name: "Иванов И. И.", Week: week1, Days: []*entities.Day{{Name: "Понедельник"}}}

	schedRepo := &mockScheduleRepo{schedules: make(map[time.Time]*entities.Schedule)}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	_ = teachRepo.Save(ctx, t1w1.Name, t1w1.Week, t1w1)

	// Новый вариант с измененным расписанием
	t1w1Updated := &entities.Teacher{Name: "Иванов И. И.", Week: week1, Days: []*entities.Day{{Name: "Вторник"}}}

	studentParser := &mockStudentParser{res: make(map[time.Time]*entities.Schedule)}
	teacherParser := &mockTeacherParser{res: []*entities.Teacher{t1w1Updated}}

	schedSvc := service.NewScheduleService(slog.Default(), schedRepo)
	scraperSvc := service.NewScraperService(
		slog.Default(),
		config.ScraperConfig{Interval: time.Hour, PeakInterval: time.Hour, NightInterval: time.Hour},
		studentParser,
		teacherParser,
		schedRepo,
		teachRepo,
		schedSvc,
	)

	eventCh := make(chan service.Event, 100)
	go scraperSvc.Start(ctx, eventCh)

	time.Sleep(100 * time.Millisecond)

	assert.Len(t, eventCh, 1, "Скрапер должен отправить ровно 1 событие при реальном изменении")
	event := <-eventCh
	assert.Equal(t, service.EventTeacherScheduleUpdated, event.Type)
	assert.Equal(t, "Иванов И. И.", event.Teacher.Name)
}
