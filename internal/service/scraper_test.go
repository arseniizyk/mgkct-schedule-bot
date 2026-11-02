package service_test

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type mockScheduleRepo struct {
	mu        sync.RWMutex
	schedules map[time.Time]*entities.Schedule
	saveCalls int
	saveErr   error
}

func (m *mockScheduleRepo) Save(ctx context.Context, week time.Time, schedule *entities.Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saveCalls++
	m.schedules[week] = schedule
	return nil
}

func (m *mockScheduleRepo) GetSaveCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveCalls
}

func (m *mockScheduleRepo) GetByWeek(ctx context.Context, week time.Time) (*entities.Schedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.schedules[week]; ok {
		return s, nil
	}
	var bestWeek time.Time
	var found bool
	for w := range m.schedules {
		if !w.After(week) {
			if !found || w.After(bestWeek) {
				bestWeek = w
				found = true
			}
		}
	}
	if found {
		return m.schedules[bestWeek], nil
	}
	return nil, nil
}

func (m *mockScheduleRepo) GetLatest(ctx context.Context) (*entities.Schedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var maxWeek time.Time
	for w := range m.schedules {
		if w.After(maxWeek) {
			maxWeek = w
		}
	}
	return m.schedules[maxWeek], nil
}

func (m *mockScheduleRepo) GetAll(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return maps.Clone(m.schedules), nil
}

func (m *mockScheduleRepo) GetAllWeeks(ctx context.Context) ([]time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var weeks []time.Time
	for w := range m.schedules {
		weeks = append(weeks, w)
	}
	slices.SortFunc(weeks, func(a, b time.Time) int {
		if a.After(b) {
			return -1
		}
		if a.Before(b) {
			return 1
		}
		return 0
	})
	return weeks, nil
}

func (m *mockScheduleRepo) GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

type mockTeacherRepo struct {
	mu       sync.RWMutex
	teachers map[string]map[time.Time]*entities.Teacher
}

func (m *mockTeacherRepo) Save(ctx context.Context, name string, week time.Time, schedule *entities.Teacher) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.teachers[name] == nil {
		m.teachers[name] = make(map[time.Time]*entities.Teacher)
	}
	m.teachers[name][week] = schedule
	return nil
}

func (m *mockTeacherRepo) GetByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.teachers[name] == nil {
		return nil, nil
	}
	return m.teachers[name][week], nil
}

func (m *mockTeacherRepo) GetLatest(ctx context.Context, name string) (*entities.Teacher, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var maxWeek time.Time
	for w := range m.teachers[name] {
		if w.After(maxWeek) {
			maxWeek = w
		}
	}
	return m.teachers[name][maxWeek], nil
}

func (m *mockTeacherRepo) GetAllLatest(ctx context.Context) (map[string]*entities.Teacher, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make(map[string]*entities.Teacher)
	for name := range m.teachers {
		t, _ := m.GetLatest(ctx, name)
		res[name] = t
	}
	return res, nil
}

func (m *mockTeacherRepo) GetAll(ctx context.Context) ([]*entities.Teacher, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []*entities.Teacher
	for _, weeks := range m.teachers {
		for _, t := range weeks {
			res = append(res, t)
		}
	}
	return res, nil
}

func (m *mockTeacherRepo) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var names []string
	for n := range m.teachers {
		names = append(names, n)
	}
	return names, nil
}

func (m *mockTeacherRepo) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	weekSet := make(map[time.Time]struct{})
	if name != "" {
		for w := range m.teachers[name] {
			weekSet[w] = struct{}{}
		}
	} else {
		for _, wMap := range m.teachers {
			for w := range wMap {
				weekSet[w] = struct{}{}
			}
		}
	}
	var weeks []time.Time
	for w := range weekSet {
		weeks = append(weeks, w)
	}
	slices.SortFunc(weeks, func(a, b time.Time) int {
		if a.After(b) {
			return -1
		}
		if a.Before(b) {
			return 1
		}
		return 0
	})
	return weeks, nil
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
	res     []*entities.Teacher
	err     error
	onParse func()
}

func (p *mockTeacherParser) Parse(ctx context.Context) ([]*entities.Teacher, error) {
	if p.onParse != nil {
		p.onParse()
	}
	return p.res, p.err
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

	futureWeek := service.CurrentWeekMonday(time.Now()).AddDate(0, 0, 7)
	t1w1 := &entities.Teacher{
		Name: "Иванов И. И.",
		Week: futureWeek,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Математика", Group: "101", Class: "204"}}},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: make(map[time.Time]*entities.Schedule)}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	_ = teachRepo.Save(ctx, t1w1.Name, t1w1.Week, t1w1)

	// Новый вариант с измененным расписанием (изменилась группа и аудитория)
	t1w1Updated := &entities.Teacher{
		Name: "Иванов И. И.",
		Week: futureWeek,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Математика", Group: "102", Class: "301"}}},
				},
			},
		},
	}

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

	select {
	case event := <-eventCh:
		assert.Equal(t, service.EventTeacherScheduleUpdated, event.Type)
		assert.Equal(t, "Иванов И. И.", event.Teacher.Name)
		assert.NotNil(t, event.Diff, "Event should contain ScheduleDiff")
		assert.NotEmpty(t, event.Diff.Days)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event in eventCh")
	}
}

type dynamicParser struct {
	mu        sync.Mutex
	failsLeft int
}

func (p *dynamicParser) ParseStudent(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failsLeft > 0 {
		return nil, assert.AnError
	}
	return make(map[time.Time]*entities.Schedule), nil
}

func (p *dynamicParser) ParseTeacher(ctx context.Context) ([]*entities.Teacher, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failsLeft > 0 {
		p.failsLeft--
		return nil, assert.AnError
	}
	return nil, nil
}

type mockFailingStudentParser struct {
	dp *dynamicParser
}

func (m *mockFailingStudentParser) Parse(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	return m.dp.ParseStudent(ctx)
}

type mockFailingTeacherParser struct {
	dp *dynamicParser
}

func (m *mockFailingTeacherParser) Parse(ctx context.Context) ([]*entities.Teacher, error) {
	return m.dp.ParseTeacher(ctx)
}

func TestScraperService_ErrorAlertAndRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	dp := &dynamicParser{failsLeft: 5}
	studentParser := &mockFailingStudentParser{dp: dp}
	teacherParser := &mockFailingTeacherParser{dp: dp}

	schedRepo := &mockScheduleRepo{schedules: make(map[time.Time]*entities.Schedule)}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	schedSvc := service.NewScheduleService(slog.Default(), schedRepo)
	scraperSvc := service.NewScraperService(
		slog.Default(),
		config.ScraperConfig{
			Interval:      10 * time.Millisecond,
			PeakInterval:  10 * time.Millisecond,
			NightInterval: 10 * time.Millisecond,
		},
		studentParser,
		teacherParser,
		schedRepo,
		teachRepo,
		schedSvc,
	)

	eventCh := make(chan service.Event, 10)
	go scraperSvc.Start(ctx, eventCh)

	// Ждем событий (ошибка после 5 падений, затем восстановление)
	var gotErrorEvent, gotRecoverEvent bool
	timeout := time.After(2 * time.Second)

	for !gotErrorEvent || !gotRecoverEvent {
		select {
		case ev := <-eventCh:
			switch ev.Type {
			case service.EventScraperError:
				gotErrorEvent = true
				assert.Equal(t, 5, ev.ConsecutiveFails)
			case service.EventScraperRecovered:
				gotRecoverEvent = true
			default:
			}
		case <-timeout:
			t.Fatalf("timeout waiting for error/recovered events: gotError=%v, gotRecover=%v", gotErrorEvent, gotRecoverEvent)
		}
	}

	assert.True(t, gotErrorEvent, "expected EventScraperError")
	assert.True(t, gotRecoverEvent, "expected EventScraperRecovered")
}

func TestScraperService_SingleParserErrorAlert(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	studentParser := &mockCountingStudentParser{failsLeft: 5}
	teacherParser := &mockTeacherParser{res: nil} // учитель всегда успешен

	schedRepo := &mockScheduleRepo{schedules: make(map[time.Time]*entities.Schedule)}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	schedSvc := service.NewScheduleService(slog.Default(), schedRepo)
	scraperSvc := service.NewScraperService(
		slog.Default(),
		config.ScraperConfig{
			Interval:      10 * time.Millisecond,
			PeakInterval:  10 * time.Millisecond,
			NightInterval: 10 * time.Millisecond,
		},
		studentParser,
		teacherParser,
		schedRepo,
		teachRepo,
		schedSvc,
	)

	eventCh := make(chan service.Event, 10)
	go scraperSvc.Start(ctx, eventCh)

	var gotErrorEvent, gotRecoverEvent bool
	timeout := time.After(2 * time.Second)

	for !gotErrorEvent || !gotRecoverEvent {
		select {
		case ev := <-eventCh:
			switch ev.Type {
			case service.EventScraperError:
				gotErrorEvent = true
				assert.Equal(t, 5, ev.ConsecutiveFails)
				require.Error(t, ev.Err)
			case service.EventScraperRecovered:
				gotRecoverEvent = true
			default:
			}
		case <-timeout:
			t.Fatalf("timeout waiting for single parser error/recovery events: gotError=%v, gotRecover=%v", gotErrorEvent, gotRecoverEvent)
		}
	}

	assert.True(t, gotErrorEvent, "expected EventScraperError when only studentParser fails")
	assert.True(t, gotRecoverEvent, "expected EventScraperRecovered")
}

type mockCountingStudentParser struct {
	mu        sync.Mutex
	failsLeft int
}

func (m *mockCountingStudentParser) Parse(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failsLeft > 0 {
		m.failsLeft--
		return nil, assert.AnError
	}
	return make(map[time.Time]*entities.Schedule), nil
}

func TestScraperService_PastChangesSavedToDBWithoutEmittingEvent(t *testing.T) {
	ctx := t.Context()

	// Изменения за прошедшую неделю
	pastWeek := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	group1 := &entities.Group{
		ID:   101,
		Week: pastWeek,
		Days: []*entities.Day{
			{Name: "Понедельник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Старый предмет"}}}}},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{
		pastWeek: {
			Groups: map[int32]*entities.Group{
				101: group1,
			},
		},
	}}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	// Новое расписание на прошедшую неделю (изменение задним числом)
	group1Updated := &entities.Group{
		ID:   101,
		Week: pastWeek,
		Days: []*entities.Day{
			{Name: "Понедельник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Новый предмет"}}}}},
		},
	}

	studentParser := &mockStudentParser{res: map[time.Time]*entities.Schedule{
		pastWeek: {
			Groups: map[int32]*entities.Group{
				101: group1Updated,
			},
		},
	}}
	teacherParser := &mockTeacherParser{res: nil}

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

	// Устанавливаем время на текущую неделю (15.10.2026)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	scraperSvc.SetNowFunc(func() time.Time { return now })

	eventCh := make(chan service.Event, 10)
	go scraperSvc.Start(ctx, eventCh)

	time.Sleep(100 * time.Millisecond)

	// Событий быть НЕ должно, так как изменения задним числом не спамят в Telegram
	assert.Empty(t, eventCh, "изменения задним числом не должны отправлять уведомления")

	// Но в репозитории данные должны обновиться!
	saved, err := schedRepo.GetByWeek(ctx, pastWeek)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, "Новый предмет", saved.Groups[101].Days[0].Subjects[0].Pairs[0].Name)
}

func TestScraperService_ProcessingOrder_DiffBeforeDBUpdate(t *testing.T) {
	ctx := t.Context()

	futureWeek := service.CurrentWeekMonday(time.Now()).AddDate(0, 0, 7)
	gOld := &entities.Group{
		ID:   102,
		Week: futureWeek,
		Days: []*entities.Day{
			{
				Name: "Вторник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Информатика", Class: "305"}}},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{
		futureWeek: {
			Groups: map[int32]*entities.Group{
				102: gOld,
			},
		},
	}}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	gNew := &entities.Group{
		ID:   102,
		Week: futureWeek,
		Days: []*entities.Day{
			{
				Name: "Вторник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Информатика", Class: "102"}}},
				},
			},
		},
	}

	studentParser := &mockStudentParser{res: map[time.Time]*entities.Schedule{
		futureWeek: {
			Groups: map[int32]*entities.Group{
				102: gNew,
			},
		},
	}}
	teacherParser := &mockTeacherParser{res: nil}

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

	eventCh := make(chan service.Event, 10)
	go scraperSvc.Start(ctx, eventCh)

	select {
	case ev := <-eventCh:
		assert.Equal(t, service.EventGroupScheduleUpdated, ev.Type)
		assert.Equal(t, int32(102), ev.Group.ID)
		require.NotNil(t, ev.Diff)
		require.Len(t, ev.Diff.Days, 1)
		require.Len(t, ev.Diff.Days[0].Pairs, 1)

		pDiff := ev.Diff.Days[0].Pairs[0]
		require.Len(t, pDiff.SubgroupDiffs, 1)
		// Проверяем, что diff сохранил старое состояние ("305") и новое ("102")
		assert.Equal(t, "305", pDiff.SubgroupDiffs[0].OldPair.Class)
		assert.Equal(t, "102", pDiff.SubgroupDiffs[0].NewPair.Class)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for group update event")
	}

	// Проверяем, что в репозитории обновленное состояние сохранено
	saved, err := schedRepo.GetByWeek(ctx, futureWeek)
	require.NoError(t, err)
	assert.Equal(t, "102", saved.Groups[102].Days[0].Subjects[0].Pairs[0].Class)
}

func TestScraperService_NoRedundantDBSaveWhenUnchanged(t *testing.T) {
	ctx := t.Context()

	currentWeek := service.CurrentWeekMonday(time.Now())
	grp := &entities.Group{
		ID:   101,
		Week: currentWeek,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Математика", Class: "201"}}},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{
		currentWeek: {
			Groups: map[int32]*entities.Group{
				101: grp,
			},
		},
	}}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	studentParser := &mockStudentParser{res: map[time.Time]*entities.Schedule{
		currentWeek: {
			Groups: map[int32]*entities.Group{
				101: grp,
			},
		},
	}}
	firstScrapeDone := make(chan struct{}, 1)
	teacherParser := &mockTeacherParser{
		res: nil,
		onParse: func() {
			select {
			case firstScrapeDone <- struct{}{}:
			default:
			}
		},
	}

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

	eventCh := make(chan service.Event, 10)
	go scraperSvc.Start(ctx, eventCh)

	select {
	case <-firstScrapeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial scrape cycle")
	}

	// Начальное состояние уже в БД, поэтому при совпадении хэшей Save не должен вызываться повторно
	assert.Equal(t, 0, schedRepo.GetSaveCalls(), "Save не должен вызываться для неизмененного расписания")
}

func TestScraperService_CacheInvalidation_NonLatestWeek(t *testing.T) {
	ctx := t.Context()

	currentWeek := service.CurrentWeekMonday(time.Now())
	nextWeek := currentWeek.AddDate(0, 0, 7)

	grpCurrent := &entities.Group{
		ID:   101,
		Week: currentWeek,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Математика", Class: "201"}}},
				},
			},
		},
	}
	grpNext := &entities.Group{
		ID:   101,
		Week: nextWeek,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Физика", Class: "301"}}},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{
		currentWeek: {Groups: map[int32]*entities.Group{101: grpCurrent}},
		nextWeek:    {Groups: map[int32]*entities.Group{101: grpNext}},
	}}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	// Имитируем обновление текущей недели при уже существующей следующей
	grpCurrentUpdated := &entities.Group{
		ID:   101,
		Week: currentWeek,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Информатика", Class: "505"}}},
				},
			},
		},
	}

	studentParser := &mockStudentParser{res: map[time.Time]*entities.Schedule{
		currentWeek: {Groups: map[int32]*entities.Group{101: grpCurrentUpdated}},
		nextWeek:    {Groups: map[int32]*entities.Group{101: grpNext}},
	}}

	scrapeDone := make(chan struct{}, 1)
	teacherParser := &mockTeacherParser{
		res: nil,
		onParse: func() {
			select {
			case scrapeDone <- struct{}{}:
			default:
			}
		},
	}

	schedSvc := service.NewScheduleService(slog.Default(), schedRepo)
	tuesdayNow := currentWeek.AddDate(0, 0, 1).Add(10 * time.Hour) // Вторник 10:00 текущей недели
	schedSvc.SetNowFunc(func() time.Time { return tuesdayNow })

	// Заполняем оба кэша (weeksCache и быстрый active s.cache) старыми данными текущей недели
	_, err := schedSvc.GetGroupScheduleByWeek(ctx, 101, currentWeek)
	require.NoError(t, err)

	latestPre, err := schedSvc.GetGroupLatestSchedule(ctx, 101)
	require.NoError(t, err)
	assert.Equal(t, "Математика", latestPre.Days[0].Subjects[0].Pairs[0].Name)

	scraperSvc := service.NewScraperService(
		slog.Default(),
		config.ScraperConfig{Interval: time.Hour, PeakInterval: time.Hour, NightInterval: time.Hour},
		studentParser,
		teacherParser,
		schedRepo,
		teachRepo,
		schedSvc,
	)

	eventCh := make(chan service.Event, 10)
	go scraperSvc.Start(ctx, eventCh)

	select {
	case <-scrapeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for scrape cycle")
	}

	// Проверяем, что weeksCache текущей недели инвалидирован и отдает обновленное значение
	cached, err := schedSvc.GetGroupScheduleByWeek(ctx, 101, currentWeek)
	require.NoError(t, err)
	assert.Equal(t, "Информатика", cached.Days[0].Subjects[0].Pairs[0].Name)

	// Проверяем, что активный кэш s.cache также инвалидирован и GetGroupLatestSchedule возвращает актуальные пары
	latestPost, err := schedSvc.GetGroupLatestSchedule(ctx, 101)
	require.NoError(t, err)
	assert.Equal(t, "Информатика", latestPost.Days[0].Subjects[0].Pairs[0].Name)
}

func TestScraperService_FailedDBSave_RetriesOnNextTick(t *testing.T) {
	ctx := t.Context()

	currentWeek := service.CurrentWeekMonday(time.Now())
	grp := &entities.Group{
		ID:   101,
		Week: currentWeek,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Математика", Class: "201"}}},
				},
			},
		},
	}

	// Инициализируем БД с ошибкой на сохранение
	schedRepo := &mockScheduleRepo{
		schedules: make(map[time.Time]*entities.Schedule),
		saveErr:   assert.AnError,
	}
	teachRepo := &mockTeacherRepo{teachers: make(map[string]map[time.Time]*entities.Teacher)}

	studentParser := &mockStudentParser{res: map[time.Time]*entities.Schedule{
		currentWeek: {Groups: map[int32]*entities.Group{101: grp}},
	}}

	scrapeCount := 0
	scrapeDone := make(chan struct{}, 5)
	teacherParser := &mockTeacherParser{
		res: nil,
		onParse: func() {
			scrapeCount++
			scrapeDone <- struct{}{}
		},
	}

	schedSvc := service.NewScheduleService(slog.Default(), schedRepo)
	scraperSvc := service.NewScraperService(
		slog.Default(),
		config.ScraperConfig{Interval: 20 * time.Millisecond, PeakInterval: 20 * time.Millisecond, NightInterval: 20 * time.Millisecond},
		studentParser,
		teacherParser,
		schedRepo,
		teachRepo,
		schedSvc,
	)

	eventCh := make(chan service.Event, 10)
	go scraperSvc.Start(ctx, eventCh)

	// Ждем первого тика (завершится ошибкой сохранения)
	select {
	case <-scrapeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first scrape")
	}

	// Хэш не должен зафиксироваться как сохраненный
	// Убираем ошибку сохранения для следующего тика
	schedRepo.mu.Lock()
	schedRepo.saveErr = nil
	schedRepo.mu.Unlock()

	// Ждем второго тика
	select {
	case <-scrapeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second scrape")
	}

	// Теперь запись должна успешно пройти в БД
	assert.GreaterOrEqual(t, schedRepo.GetSaveCalls(), 1, "Save должен повториться на следующем тике после сбоя")
}
