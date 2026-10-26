package handlers_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	userGroup    map[int64]int
	userSubgroup map[int64]int
	userTeacher  map[int64]string
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		userGroup:    make(map[int64]int),
		userSubgroup: make(map[int64]int),
		userTeacher:  make(map[int64]string),
	}
}

func (m *mockUserRepo) SaveUser(ctx context.Context, u entities.User) error {
	return nil
}

func (m *mockUserRepo) GroupByChatID(ctx context.Context, chatID int64) (int, error) {
	return m.userGroup[chatID], nil
}

func (m *mockUserRepo) SubgroupByChatID(ctx context.Context, chatID int64) (int, error) {
	return m.userSubgroup[chatID], nil
}

func (m *mockUserRepo) GroupAndSubgroupByChatID(ctx context.Context, chatID int64) (int, int, error) {
	return m.userGroup[chatID], m.userSubgroup[chatID], nil
}

func (m *mockUserRepo) SetUserGroup(ctx context.Context, chatID int64, groupID int, subgroup int) error {
	m.userGroup[chatID] = groupID
	m.userSubgroup[chatID] = subgroup
	delete(m.userTeacher, chatID)
	return nil
}

func (m *mockUserRepo) GetTeacher(ctx context.Context, chatID int64) (string, error) {
	return m.userTeacher[chatID], nil
}

func (m *mockUserRepo) SetTeacher(ctx context.Context, chatID int64, teacherName string) error {
	m.userTeacher[chatID] = teacherName
	delete(m.userGroup, chatID)
	delete(m.userSubgroup, chatID)
	return nil
}

func (m *mockUserRepo) SetNotifyEvening(ctx context.Context, chatID int64, enabled bool) error {
	return nil
}

func (m *mockUserRepo) GetNotifyEvening(ctx context.Context, chatID int64) (bool, error) {
	return true, nil
}

type mockContext struct {
	tele.Context

	args       []string
	text       string
	chatID     int64
	sentWhat   any
	sentOpts   []any
	editedWhat any
	callback   *tele.Callback
}

func (c *mockContext) Args() []string {
	return c.args
}

func (c *mockContext) Text() string {
	return c.text
}

func (c *mockContext) Chat() *tele.Chat {
	return &tele.Chat{ID: c.chatID}
}

func (c *mockContext) Sender() *tele.User {
	return &tele.User{ID: c.chatID, Username: "testuser"}
}

func (c *mockContext) Send(what any, opts ...any) error {
	c.sentWhat = what
	c.sentOpts = opts
	return nil
}

func (c *mockContext) Edit(what any, opts ...any) error {
	c.editedWhat = what
	return nil
}

func (c *mockContext) Callback() *tele.Callback {
	return c.callback
}

func (c *mockContext) Respond(resp ...*tele.CallbackResponse) error {
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestSetGroupHandler(t *testing.T) {
	logger := testLogger()
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()

	handler := handlers.SetGroup(logger, repo, sm)

	t.Run("without args sets waiting_group state", func(t *testing.T) {
		ctx := &mockContext{chatID: 100}
		err := handler(ctx)
		require.NoError(t, err)
		assert.Equal(t, messages.WaitingGroup, ctx.sentWhat)

		st, ok := sm.Get(100)
		assert.True(t, ok)
		assert.Equal(t, state.WaitingGroup, st)
	})

	t.Run("with single group arg asks for subgroup", func(t *testing.T) {
		ctx := &mockContext{chatID: 101, args: []string{"101"}}
		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.sentWhat, "101")
		require.NotEmpty(t, ctx.sentOpts)
		_, ok := ctx.sentOpts[0].(*tele.ReplyMarkup)
		assert.True(t, ok)
	})

	t.Run("with group and subgroup args saves directly", func(t *testing.T) {
		ctx := &mockContext{chatID: 102, args: []string{"102", "1"}}
		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.sentWhat, "102")
		assert.Contains(t, ctx.sentWhat, "1 подгруппа")

		grp, sg, err := repo.GroupAndSubgroupByChatID(context.Background(), 102)
		require.NoError(t, err)
		assert.Equal(t, 102, grp)
		assert.Equal(t, 1, sg)
	})
}

func TestStatesHandler_WaitingGroup(t *testing.T) {
	logger := testLogger()
	repo := newMockUserRepo()
	sm := state.NewMemoryManager()

	handler := handlers.StatesHandler(logger, repo, sm, nil)

	t.Run("user inputs only group number", func(t *testing.T) {
		_ = sm.Set(201, state.WaitingGroup)
		ctx := &mockContext{chatID: 201, text: "101"}
		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.sentWhat, "101")

		// Состояние должно быть очищено
		_, ok := sm.Get(201)
		assert.False(t, ok)
	})

	t.Run("user inputs group and subgroup", func(t *testing.T) {
		_ = sm.Set(202, state.WaitingGroup)
		ctx := &mockContext{chatID: 202, text: "102 2"}
		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.sentWhat, "2 подгруппа")

		grp, sg, err := repo.GroupAndSubgroupByChatID(context.Background(), 202)
		require.NoError(t, err)
		assert.Equal(t, 102, grp)
		assert.Equal(t, 2, sg)

		_, ok := sm.Get(202)
		assert.False(t, ok)
	})
}

func TestCallbacksHandler_SubgroupSelect(t *testing.T) {
	logger := testLogger()
	repo := newMockUserRepo()

	handler := handlers.CallbacksHandler(logger, nil, nil, nil, repo)

	ctx := &mockContext{
		chatID: 301,
		callback: &tele.Callback{
			Data: "\f" + keyboard.SubgroupSelect + "|101:1",
		},
	}

	err := handler(ctx)
	require.NoError(t, err)

	grp, sg, err := repo.GroupAndSubgroupByChatID(context.Background(), 301)
	require.NoError(t, err)
	assert.Equal(t, 101, grp)
	assert.Equal(t, 1, sg)

	assert.Contains(t, ctx.editedWhat, "1 подгруппа")
	assert.Nil(t, ctx.sentWhat)
}

type mockScheduleService struct {
	group *entities.Group
}

func (m *mockScheduleService) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	return m.group, nil
}
func (m *mockScheduleService) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	return m.group, nil
}
func (m *mockScheduleService) GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}
func (m *mockScheduleService) GetAllWeeks(ctx context.Context) ([]time.Time, error) {
	return nil, nil
}
func (m *mockScheduleService) GetArchiveSchedule(ctx context.Context, groupID int32, targetDate time.Time) (*entities.Group, error) {
	return m.group, nil
}

type mockTeacherService struct {
	teacher *entities.Teacher
}

func (m *mockTeacherService) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	return m.teacher, nil
}
func (m *mockTeacherService) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	return m.teacher, nil
}
func (m *mockTeacherService) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}
func (m *mockTeacherService) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return []string{m.teacher.Name}, nil
}
func (m *mockTeacherService) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	return nil, nil
}
func (m *mockTeacherService) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	return m.teacher, nil
}
func (m *mockTeacherService) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	return m.teacher.Name, nil, true
}
func (m *mockTeacherService) FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error) {
	return nil, nil
}
func (m *mockTeacherService) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	return nil, nil
}

func TestCallbacksHandler_DaySubgroupAndTeacher(t *testing.T) {
	logger := testLogger()
	repo := newMockUserRepo()

	sampleGroup := &entities.Group{
		ID: 101,
		Days: []*entities.Day{
			{
				Name: "Понедельник | 26.10.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "1. Физика", Group: "Иванов", Class: "101"},
							{Name: "2. Химия", Group: "Петров", Class: "102"},
						},
					},
				},
			},
		},
	}
	schedSvc := &mockScheduleService{group: sampleGroup}

	sampleTeacher := &entities.Teacher{
		Name: "Иванов И. И.",
		Days: []*entities.Day{
			{
				Name: "Понедельник | 26.10.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Физика", Group: "101", Class: "101"},
						},
					},
				},
			},
		},
	}
	teacherSvc := &mockTeacherService{teacher: sampleTeacher}

	handler := handlers.CallbacksHandler(logger, schedSvc, teacherSvc, nil, repo)

	t.Run("group day subgroup switch to subgroup 2", func(t *testing.T) {
		ctx := &mockContext{
			chatID: 301,
			callback: &tele.Callback{
				Data: "\f" + keyboard.DaySubgroup + "|101:0:2",
			},
		}

		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "Химия")
		assert.NotContains(t, ctx.editedWhat, "Физика")
	})

	t.Run("group day subgroup switch to subgroup 1", func(t *testing.T) {
		ctx := &mockContext{
			chatID: 301,
			callback: &tele.Callback{
				Data: "\f" + keyboard.DaySubgroup + "|101:0:1",
			},
		}

		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "Физика")
		assert.NotContains(t, ctx.editedWhat, "Химия")
	})

	t.Run("teacher day callback routes to teacher handler properly", func(t *testing.T) {
		ctx := &mockContext{
			chatID: 401,
			callback: &tele.Callback{
				Data: "\f" + keyboard.TeacherPrevDay + "|Иванов И. И.:0",
			},
		}

		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "Физика")
		assert.Contains(t, ctx.editedWhat, "101")
	})

	t.Run("teacher current week callback routes to teacher handler properly", func(t *testing.T) {
		ctx := &mockContext{
			chatID: 401,
			callback: &tele.Callback{
				Data: "\f" + keyboard.TeacherCurrentWeek + "|Иванов И. И.",
			},
		}

		err := handler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "Физика")
	})
}
