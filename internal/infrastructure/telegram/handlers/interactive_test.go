package handlers_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

type mockTeacherSvcFull struct {
	validateMatched    string
	validateCandidates []string
	validateOk         bool
	teacherSchedule    *entities.Teacher
	scheduleErr        error
	allWeeks           []time.Time
}

func (m *mockTeacherSvcFull) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	if m.scheduleErr != nil {
		return nil, m.scheduleErr
	}
	if m.teacherSchedule != nil {
		return m.teacherSchedule, nil
	}
	return &entities.Teacher{Name: name, Days: []*entities.Day{{Name: "Понедельник"}}}, nil
}

func (m *mockTeacherSvcFull) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	return nil, nil
}

func (m *mockTeacherSvcFull) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

func (m *mockTeacherSvcFull) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockTeacherSvcFull) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	return m.allWeeks, nil
}

func (m *mockTeacherSvcFull) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	return nil, nil
}

func (m *mockTeacherSvcFull) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	return m.validateMatched, m.validateCandidates, m.validateOk
}

func (m *mockTeacherSvcFull) FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error) {
	return nil, nil
}

func (m *mockTeacherSvcFull) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	return &entities.FacultyMember{FullName: teacherName}, nil
}

type mockAnalyticsSvc struct {
	cabSched   *entities.CabinetSchedule
	candidates []string
	cabErr     error
}

func (m *mockAnalyticsSvc) GetAllCabinets(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockAnalyticsSvc) GetCabinetSchedule(ctx context.Context, query string, week time.Time) (*entities.CabinetSchedule, []string, error) {
	return m.cabSched, m.candidates, m.cabErr
}

func (m *mockAnalyticsSvc) GetEndings(ctx context.Context, dayIdx int, week time.Time) (*entities.EndingsStat, error) {
	return nil, nil
}

func (m *mockAnalyticsSvc) GetGroupStats(ctx context.Context, groupID int32, week time.Time) ([]entities.SubjectStat, int, error) {
	return nil, 0, nil
}

func (m *mockAnalyticsSvc) GetTeacherStats(ctx context.Context, teacherName string, week time.Time) ([]entities.SubjectStat, int, error) {
	return nil, 0, nil
}

type mockSubjectDict struct {
	lookupFullName string
	lookupOk       bool
	resolveCleaned string
	resolveFull    string
	resolveFound   bool
	searchResult   []entities.SubjectInfo
}

func (m *mockSubjectDict) Lookup(shortName string) (string, bool) {
	return m.lookupFullName, m.lookupOk
}

func (m *mockSubjectDict) Resolve(raw string) (string, string, bool) {
	return m.resolveCleaned, m.resolveFull, m.resolveFound
}

func (m *mockSubjectDict) All() map[string]string {
	return nil
}

func (m *mockSubjectDict) Search(query string) []entities.SubjectInfo {
	return m.searchResult
}

type mockScheduleSvc struct {
	weeks []time.Time
}

func (m *mockScheduleSvc) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	return nil, nil
}

func (m *mockScheduleSvc) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	return nil, nil
}

func (m *mockScheduleSvc) GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

func (m *mockScheduleSvc) GetAllWeeks(ctx context.Context) ([]time.Time, error) {
	return m.weeks, nil
}

func (m *mockScheduleSvc) GetArchiveSchedule(ctx context.Context, groupID int32, targetDate time.Time) (*entities.Group, error) {
	return &entities.Group{ID: groupID, Days: []*entities.Day{{Name: "Понедельник"}}}, nil
}

func testLog() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestInteractive_TeacherDay_NoTeacherSet_WaitingState(t *testing.T) {
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()
	tSvc := &mockTeacherSvcFull{}

	h := handlers.TeacherDay(testLog(), tSvc, repo, sm)
	ctx := &mockContext{chatID: 100}

	err := h(ctx)
	require.NoError(t, err)

	assert.Equal(t, messages.WaitingTeacherDay, ctx.sentWhat)
	st, ok := sm.Get(100)
	assert.True(t, ok)
	assert.Equal(t, state.WaitingTeacherDay, st)
}

func TestInteractive_TeacherDay_WithTeacherSet_ShowsSchedule(t *testing.T) {
	repo := newMockUserRepo()
	_ = repo.SetTeacher(context.Background(), 101, "Иванов И. И.")
	sm := state.NewMemoryManager()
	tSvc := &mockTeacherSvcFull{
		teacherSchedule: &entities.Teacher{Name: "Иванов И. И.", Days: []*entities.Day{{Name: "Понедельник"}}},
	}

	h := handlers.TeacherDay(testLog(), tSvc, repo, sm)
	ctx := &mockContext{chatID: 101}

	err := h(ctx)
	require.NoError(t, err)

	_, ok := sm.Get(101)
	assert.False(t, ok)
	assert.Contains(t, ctx.sentWhat, "Иванов И. И.")
}

func TestInteractive_Cabinet_Empty_WaitingState(t *testing.T) {
	sm := state.NewMemoryManager()
	aSvc := &mockAnalyticsSvc{}

	h := handlers.Cabinet(testLog(), aSvc, sm)
	ctx := &mockContext{chatID: 200}

	err := h(ctx)
	require.NoError(t, err)

	assert.Equal(t, messages.WaitingCabinet, ctx.sentWhat)
	st, ok := sm.Get(200)
	assert.True(t, ok)
	assert.Equal(t, state.WaitingCabinet, st)
}

func TestInteractive_Subject_Empty_WaitingState(t *testing.T) {
	sm := state.NewMemoryManager()
	dict := &mockSubjectDict{}

	h := handlers.Subject(dict, sm)
	ctx := &mockContext{chatID: 300}

	err := h(ctx)
	require.NoError(t, err)

	assert.Equal(t, messages.WaitingSubject, ctx.sentWhat)
	st, ok := sm.Get(300)
	assert.True(t, ok)
	assert.Equal(t, state.WaitingSubject, st)
}

func TestInteractive_Archive_NoProfile_WaitingState(t *testing.T) {
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()
	schedSvc := &mockScheduleSvc{}
	tSvc := &mockTeacherSvcFull{}

	h := handlers.Archive(testLog(), schedSvc, tSvc, repo, sm)
	ctx := &mockContext{chatID: 400}

	err := h(ctx)
	require.NoError(t, err)

	assert.Equal(t, messages.WaitingArchive, ctx.sentWhat)
	st, ok := sm.Get(400)
	assert.True(t, ok)
	assert.Equal(t, state.WaitingArchive, st)
}

func TestInteractive_StatesHandler_WaitingTeacherDay(t *testing.T) {
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()
	tSvc := &mockTeacherSvcFull{
		validateMatched: "Петров П. П.",
		validateOk:      true,
		teacherSchedule: &entities.Teacher{Name: "Петров П. П.", Days: []*entities.Day{{Name: "Понедельник"}}},
	}

	_ = sm.Set(500, state.WaitingTeacherDay)
	h := handlers.StatesHandler(testLog(), repo, sm, tSvc, nil, nil, nil)
	ctx := &mockContext{chatID: 500, text: "Петров"}

	err := h(ctx)
	require.NoError(t, err)

	_, ok := sm.Get(500)
	assert.False(t, ok, "state must be cleared after handling")
	assert.Contains(t, ctx.sentWhat, "Петров П. П.")
}

func TestInteractive_StatesHandler_WaitingCabinet(t *testing.T) {
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()
	aSvc := &mockAnalyticsSvc{
		cabSched: &entities.CabinetSchedule{Cabinet: "301", Days: []*entities.CabinetDaySchedule{{DayName: "Понедельник"}}},
	}

	_ = sm.Set(600, state.WaitingCabinet)
	h := handlers.StatesHandler(testLog(), repo, sm, nil, nil, aSvc, nil)
	ctx := &mockContext{chatID: 600, text: "301"}

	err := h(ctx)
	require.NoError(t, err)

	_, ok := sm.Get(600)
	assert.False(t, ok, "state must be cleared after handling")
	assert.Contains(t, ctx.sentWhat, "301")
}

func TestInteractive_StatesHandler_WaitingSubject(t *testing.T) {
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()
	dict := &mockSubjectDict{
		resolveCleaned: "ТОЭ",
		resolveFull:    "Теоретические основы электротехники",
		resolveFound:   true,
	}

	_ = sm.Set(700, state.WaitingSubject)
	h := handlers.StatesHandler(testLog(), repo, sm, nil, nil, nil, dict)
	ctx := &mockContext{chatID: 700, text: "ТОЭ"}

	err := h(ctx)
	require.NoError(t, err)

	_, ok := sm.Get(700)
	assert.False(t, ok, "state must be cleared after handling")
	assert.Contains(t, ctx.sentWhat, "Теоретические основы электротехники")
}

func TestInteractive_StatesHandler_WaitingArchive_Group(t *testing.T) {
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()
	schedSvc := &mockScheduleSvc{
		weeks: []time.Time{time.Now()},
	}

	_ = sm.Set(800, state.WaitingArchive)
	h := handlers.StatesHandler(testLog(), repo, sm, nil, schedSvc, nil, nil)
	ctx := &mockContext{chatID: 800, text: "101"}

	err := h(ctx)
	require.NoError(t, err)

	_, ok := sm.Get(800)
	assert.False(t, ok, "state must be cleared after handling")
	assert.Contains(t, ctx.sentWhat, "101")
}

func TestInteractive_StatesHandler_WaitingArchive_Teacher(t *testing.T) {
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()
	tSvc := &mockTeacherSvcFull{
		validateMatched: "Иванов И. И.",
		validateOk:      true,
		allWeeks:        []time.Time{time.Now()},
	}

	_ = sm.Set(900, state.WaitingArchive)
	h := handlers.StatesHandler(testLog(), repo, sm, tSvc, nil, nil, nil)
	ctx := &mockContext{chatID: 900, text: "Иванов"}

	err := h(ctx)
	require.NoError(t, err)

	_, ok := sm.Get(900)
	assert.False(t, ok, "state must be cleared after handling")
	assert.Contains(t, ctx.sentWhat, "Иванов И. И.")
}

func TestInteractive_Cancel(t *testing.T) {
	sm := state.NewMemoryManager()
	_ = sm.Set(1000, state.WaitingCabinet)

	h := handlers.Cancel(testLog(), sm)
	ctx := &mockContext{chatID: 1000}

	err := h(ctx)
	require.NoError(t, err)

	_, ok := sm.Get(1000)
	assert.False(t, ok, "state must be cleared on cancel")
	assert.Equal(t, messages.Cancelled, ctx.sentWhat)
}

func TestInteractive_ArchiveSelect_And_Back(t *testing.T) {
	schedSvc := &mockScheduleSvc{
		weeks: []time.Time{time.Now()},
	}
	tSvc := &mockTeacherSvcFull{}
	userRepo := newMockUserRepo()
	sm := state.NewMemoryManager()

	cbHandler := handlers.CallbacksHandler(testLog(), schedSvc, tSvc, nil, userRepo, sm)

	// 1. Выбор недели в архиве: arch_sel|g:88:19.10.2026
	ctxSelect := &mockContext{
		chatID:   1234,
		callback: &tele.Callback{Data: "\x0carch_sel|g:88:19.10.2026"},
	}

	err := cbHandler(ctxSelect)
	require.NoError(t, err)
	assert.NotNil(t, ctxSelect.editedWhat)

	// Проверяем, что передана Inline-клавиатура возврата, а не ReplyScheduleKeyboard
	var hasReturnInline bool
	for _, opt := range ctxSelect.editedOpts {
		if rm, ok := opt.(*tele.ReplyMarkup); ok {
			if len(rm.InlineKeyboard) > 0 && rm.InlineKeyboard[0][0].Text == "📦 К выбору недели" {
				hasReturnInline = true
				break
			}
		}
	}
	assert.True(t, hasReturnInline, "must have inline button 'К выбору недели'")

	// 2. Нажатие кнопки возврата к выбору недели: arch_back|g:88
	ctxBack := &mockContext{
		chatID:   1234,
		callback: &tele.Callback{Data: "\x0carch_back|g:88"},
	}

	err = cbHandler(ctxBack)
	require.NoError(t, err)
	assert.Contains(t, ctxBack.editedWhat, "Архив расписания группы 88")
}
