package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

func TestCalculateTomorrowDate(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)

	tests := []struct {
		name       string
		now        time.Time
		wantDate   string
		wantDayIdx int
	}{
		{
			name:       "Monday -> Tuesday (idx 1)",
			now:        time.Date(2026, 10, 12, 20, 0, 0, 0, minskLoc),
			wantDate:   "13.10.2026",
			wantDayIdx: 1,
		},
		{
			name:       "Tuesday -> Wednesday (idx 2)",
			now:        time.Date(2026, 10, 13, 20, 0, 0, 0, minskLoc),
			wantDate:   "14.10.2026",
			wantDayIdx: 2,
		},
		{
			name:       "Wednesday -> Thursday (idx 3)",
			now:        time.Date(2026, 10, 14, 20, 0, 0, 0, minskLoc),
			wantDate:   "15.10.2026",
			wantDayIdx: 3,
		},
		{
			name:       "Thursday -> Friday (idx 4)",
			now:        time.Date(2026, 10, 15, 20, 0, 0, 0, minskLoc),
			wantDate:   "16.10.2026",
			wantDayIdx: 4,
		},
		{
			name:       "Friday -> Saturday (idx 5)",
			now:        time.Date(2026, 10, 16, 20, 0, 0, 0, minskLoc),
			wantDate:   "17.10.2026",
			wantDayIdx: 5,
		},
		{
			name:       "Saturday -> Monday (idx 0)",
			now:        time.Date(2026, 10, 17, 20, 0, 0, 0, minskLoc),
			wantDate:   "19.10.2026",
			wantDayIdx: 0,
		},
		{
			name:       "Sunday -> Monday (idx 0)",
			now:        time.Date(2026, 10, 18, 20, 0, 0, 0, minskLoc),
			wantDate:   "19.10.2026",
			wantDayIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDate, gotDayIdx := CalculateTomorrowDate(tt.now)
			if gotDate.Format("02.01.2006") != tt.wantDate {
				t.Errorf("gotDate = %s, want %s", gotDate.Format("02.01.2006"), tt.wantDate)
			}
			if gotDayIdx != tt.wantDayIdx {
				t.Errorf("gotDayIdx = %d, want %d", gotDayIdx, tt.wantDayIdx)
			}
		})
	}
}

func TestNotifier_ScraperAlerts_NoAdmin(t *testing.T) {
	n := &Notifier{
		adminIDs: nil,
	}

	ctx := context.Background()
	if err := n.HandleScraperError(ctx, nil, 5); err != nil {
		t.Errorf("expected nil error when adminIDs is empty, got %v", err)
	}
	if err := n.HandleScraperRecovered(ctx); err != nil {
		t.Errorf("expected nil error when adminIDs is empty, got %v", err)
	}
}

type mockBotSender struct {
	sentChats []int64
	sentWhat  []any
	sentOpts  [][]any
}

func (m *mockBotSender) Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error) {
	if chatID, err := strconv.ParseInt(to.Recipient(), 10, 64); err == nil {
		m.sentChats = append(m.sentChats, chatID)
	}
	m.sentWhat = append(m.sentWhat, what)
	m.sentOpts = append(m.sentOpts, opts)
	return &tele.Message{}, nil
}

type mockUserStore struct {
	users []entities.User
}

func (m *mockUserStore) AllUsers(ctx context.Context) ([]entities.User, error) {
	return m.users, nil
}

func (m *mockUserStore) UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error) {
	return nil, nil
}

func (m *mockUserStore) UserIDsByTeacherName(ctx context.Context, teacherName string) ([]int64, error) {
	return nil, nil
}

func (m *mockUserStore) UsersForEveningNotification(ctx context.Context) ([]entities.User, error) {
	return nil, nil
}

func (m *mockUserStore) SetNotifyEvening(ctx context.Context, chatID int64, enabled bool) error {
	return nil
}

func TestNotifier_HandleNewWeek(t *testing.T) {
	bot := &mockBotSender{}
	store := &mockUserStore{
		users: []entities.User{
			{ChatID: 1001, Group: 101},
			{ChatID: 1002, TeacherName: "Иванов И. И."},
			{ChatID: 1003},
		},
	}
	n := New(slog.Default(), bot, store, nil, rate.NewLimiter(rate.Inf, 0))

	targetWeek := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()

	err := n.HandleNewWeek(ctx, targetWeek)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bot.sentChats) != 3 {
		t.Fatalf("expected 3 sent messages, got %d", len(bot.sentChats))
	}

	// 1. Студент (Group 101) - кнопка должна вести на NextWeek с датой новой недели
	if bot.sentWhat[0] != messages.WeekUpdated {
		t.Errorf("expected %s, got %v", messages.WeekUpdated, bot.sentWhat[0])
	}
	opts1 := bot.sentOpts[0]
	if len(opts1) != 1 {
		t.Fatalf("expected 1 option (markup), got %d", len(opts1))
	}
	kbd1, ok := opts1[0].(*tele.ReplyMarkup)
	if !ok || len(kbd1.InlineKeyboard) != 1 || len(kbd1.InlineKeyboard[0]) != 1 {
		t.Fatalf("expected 1 inline button for student, got %+v", kbd1)
	}
	btn1 := kbd1.InlineKeyboard[0][0]
	if btn1.Unique != keyboard.NextWeek || btn1.Data != "101:02.11.2026" {
		t.Errorf("expected NextWeek|101:02.11.2026, got %s|%s", btn1.Unique, btn1.Data)
	}

	// 2. Преподаватель - кнопка должна вести на TeacherNextWeek с датой новой недели
	if bot.sentWhat[1] != messages.TeacherWeekUpdated {
		t.Errorf("expected %s, got %v", messages.TeacherWeekUpdated, bot.sentWhat[1])
	}
	opts2 := bot.sentOpts[1]
	kbd2, ok := opts2[0].(*tele.ReplyMarkup)
	if !ok || len(kbd2.InlineKeyboard) != 1 || len(kbd2.InlineKeyboard[0]) != 1 {
		t.Fatalf("expected 1 inline button for teacher, got %+v", kbd2)
	}
	btn2 := kbd2.InlineKeyboard[0][0]
	if btn2.Unique != keyboard.TeacherNextWeek || btn2.Data != "Иванов И. И.:02.11.2026" {
		t.Errorf("expected TeacherNextWeek|Иванов И. И.:02.11.2026, got %s|%s", btn2.Unique, btn2.Data)
	}

	// 3. Пользователь без роли
	if bot.sentWhat[2] != messages.WeekUpdated {
		t.Errorf("expected %s, got %v", messages.WeekUpdated, bot.sentWhat[2])
	}
	if len(bot.sentOpts[2]) != 0 {
		t.Errorf("expected no inline buttons for user without role, got %v", bot.sentOpts[2])
	}
}

func TestNotifier_ScraperAlerts_MultipleAdmins(t *testing.T) {
	bot := &mockBotSender{}
	n := &Notifier{
		bot:      bot,
		adminIDs: []int64{111, 222},
	}

	ctx := context.Background()
	if err := n.HandleScraperError(ctx, errors.New("timeout"), 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bot.sentChats) != 2 || bot.sentChats[0] != 111 || bot.sentChats[1] != 222 {
		t.Errorf("expected sentChats to be [111, 222], got %v", bot.sentChats)
	}

	bot.sentChats = nil
	if err := n.HandleScraperError(ctx, nil, 1); err != nil {
		t.Fatalf("unexpected error on nil err: %v", err)
	}
	if len(bot.sentChats) != 2 {
		t.Errorf("expected 2 alerts sent even when err is nil, got %d", len(bot.sentChats))
	}

	bot.sentChats = nil
	if err := n.HandleScraperRecovered(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bot.sentChats) != 2 || bot.sentChats[0] != 111 || bot.sentChats[1] != 222 {
		t.Errorf("expected sentChats to be [111, 222], got %v", bot.sentChats)
	}
}

func TestIsSameWeek(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)
	monday := time.Date(2026, 10, 12, 10, 0, 0, 0, minskLoc)
	wednesday := time.Date(2026, 10, 14, 15, 0, 0, 0, minskLoc)
	nextMonday := time.Date(2026, 10, 19, 10, 0, 0, 0, minskLoc)

	if !isSameWeek(monday, wednesday) {
		t.Errorf("expected monday and wednesday of the same week to be same week")
	}
	if isSameWeek(monday, nextMonday) {
		t.Errorf("expected monday and next monday to be different weeks")
	}
}

type mockSchedProvider struct {
	byWeekFunc func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error)
	latestFunc func(ctx context.Context, groupID int32) (*entities.Group, error)
}

func (m *mockSchedProvider) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	if m.byWeekFunc != nil {
		return m.byWeekFunc(ctx, groupID, week)
	}
	return nil, nil
}

func (m *mockSchedProvider) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	if m.latestFunc != nil {
		return m.latestFunc(ctx, groupID)
	}
	return nil, nil
}

type mockTeacherProvider struct {
	byWeekFunc func(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	latestFunc func(ctx context.Context, name string) (*entities.Teacher, error)
	memberFunc func(ctx context.Context, teacherName string) (*entities.FacultyMember, error)
}

func (m *mockTeacherProvider) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	if m.byWeekFunc != nil {
		return m.byWeekFunc(ctx, name, week)
	}
	return nil, nil
}

func (m *mockTeacherProvider) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	if m.latestFunc != nil {
		return m.latestFunc(ctx, name)
	}
	return nil, nil
}

func (m *mockTeacherProvider) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	if m.memberFunc != nil {
		return m.memberFunc(ctx, teacherName)
	}
	return nil, nil
}

func TestFormatNextDayDigest_NeitherGroupNorTeacher(t *testing.T) {
	ctx := context.Background()
	_, err := FormatNextDayDigest(ctx, entities.User{}, time.Now(), 0, nil, nil)
	if err == nil {
		t.Fatalf("expected error when neither group nor teacher specified")
	}
}

func TestFormatNextDayDigest_GroupSuccess(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 13, 12, 0, 0, 0, minskLoc) // Tuesday
	targetMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return &entities.Group{
				ID:   groupID,
				Week: targetMonday,
				Days: []*entities.Day{
					{Name: "Понедельник"},
					{
						Name: "Вторник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Математика", Class: "101"}}},
						},
					},
				},
			}, nil
		},
	}

	user := entities.User{ChatID: 12345, Group: 101}
	msg, err := FormatNextDayDigest(ctx, user, targetDate, 1, schedProv, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Расписание на") || !strings.Contains(msg, "13.10.2026") {
		t.Errorf("expected header with date, got %s", msg)
	}
	if !strings.Contains(msg, "Математика") {
		t.Errorf("expected subject in message, got %s", msg)
	}
}

func TestFormatNextDayDigest_GroupSubgroupFiltered(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 13, 12, 0, 0, 0, minskLoc)
	targetMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return &entities.Group{
				ID:   groupID,
				Week: targetMonday,
				Days: []*entities.Day{
					{Name: "Понедельник"},
					{
						Name: "Вторник",
						Subjects: []*entities.Subject{
							{
								Pairs: []*entities.Pair{
									{Name: "1. Физика", Class: "101"},
									{Name: "2. Химия", Class: "102"},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	userSub1 := entities.User{ChatID: 12345, Group: 101, Subgroup: 1}
	msgSub1, err := FormatNextDayDigest(ctx, userSub1, targetDate, 1, schedProv, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msgSub1, "Физика") {
		t.Errorf("expected subgroup 1 subject in message, got %s", msgSub1)
	}
	if strings.Contains(msgSub1, "Химия") {
		t.Errorf("did not expect subgroup 2 subject in message, got %s", msgSub1)
	}
}

func TestFormatNextDayDigest_TeacherSuccess(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 14, 12, 0, 0, 0, minskLoc) // Wednesday
	targetMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	teacherProv := &mockTeacherProvider{
		byWeekFunc: func(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
			return &entities.Teacher{
				Name: name,
				Week: targetMonday,
				Days: []*entities.Day{
					{Name: "Понедельник"},
					{Name: "Вторник"},
					{
						Name: "Среда",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Физика", Group: "101", Class: "202"}}},
						},
					},
				},
			}, nil
		},
		memberFunc: func(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
			return &entities.FacultyMember{FullName: teacherName, Department: "Кафедра физики"}, nil
		},
	}

	user := entities.User{ChatID: 12345, TeacherName: "Иванов И. И."}
	msg, err := FormatNextDayDigest(ctx, user, targetDate, 2, nil, teacherProv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Расписание на") || !strings.Contains(msg, "14.10.2026") {
		t.Errorf("expected header with date, got %s", msg)
	}
	if !strings.Contains(msg, "Физика") {
		t.Errorf("expected subject in message, got %s", msg)
	}
}

func TestFormatNextDayDigest_WeekNotPublishedYet(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 19, 12, 0, 0, 0, minskLoc) // Next Monday
	oldMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)   // Current Monday

	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return nil, errNotFound
		},
		latestFunc: func(ctx context.Context, groupID int32) (*entities.Group, error) {
			return &entities.Group{
				ID:   groupID,
				Week: oldMonday, // old week!
				Days: []*entities.Day{
					{Name: "Понедельник"},
				},
			}, nil
		},
	}

	user := entities.User{ChatID: 12345, Group: 101}
	_, err := FormatNextDayDigest(ctx, user, targetDate, 0, schedProv, nil)
	if err == nil {
		t.Fatalf("expected error when next week is not published yet, got nil")
	}
	if !strings.Contains(err.Error(), "not published yet") {
		t.Errorf("expected 'not published yet' error, got %v", err)
	}
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

type floodMockBot struct {
	attempts int
}

func (m *floodMockBot) Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error) {
	m.attempts++
	if m.attempts == 1 {
		return nil, tele.FloodError{
			RetryAfter: 0,
		}
	}
	return &tele.Message{}, nil
}

func TestNotifier_FloodWaitRetry(t *testing.T) {
	bot := &floodMockBot{}
	store := &mockUserStore{
		users: []entities.User{
			{ChatID: 1001, Group: 101},
		},
	}
	n := New(slog.Default(), bot, store, nil, rate.NewLimiter(rate.Inf, 0))

	err := n.HandleNewWeek(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("expected nil error after retry, got %v", err)
	}
	if bot.attempts != 2 {
		t.Errorf("expected 2 attempts after flood wait retry, got %d", bot.attempts)
	}
}

type forbiddenMockBot struct{}

func (m *forbiddenMockBot) Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error) {
	return nil, tele.ErrBlockedByUser
}

type spyUserStore struct {
	mockUserStore

	deactivatedChatID int64
}

func (s *spyUserStore) SetNotifyEvening(ctx context.Context, chatID int64, enabled bool) error {
	if !enabled {
		s.deactivatedChatID = chatID
	}
	return nil
}

func (s *spyUserStore) UsersForEveningNotification(ctx context.Context) ([]entities.User, error) {
	return []entities.User{{ChatID: 1002, Group: 101}}, nil
}

func TestNotifier_ForbiddenDeactivatesNotifications(t *testing.T) {
	bot := &forbiddenMockBot{}
	store := &spyUserStore{}
	n := New(slog.Default(), bot, store, nil, rate.NewLimiter(rate.Inf, 0))

	targetDate, _ := CalculateTomorrowDate(time.Now())
	days := make([]*entities.Day, 6)
	for i := range days {
		days[i] = &entities.Day{
			Name: "День",
			Subjects: []*entities.Subject{
				{Pairs: []*entities.Pair{{Name: "Математика"}}},
			},
		}
	}
	targetMonday := targetDate.AddDate(0, 0, -int(targetDate.Weekday()-time.Monday))

	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return &entities.Group{ID: 101, Week: targetMonday, Days: days}, nil
		},
		latestFunc: func(ctx context.Context, groupID int32) (*entities.Group, error) {
			return &entities.Group{ID: 101, Week: targetMonday, Days: days}, nil
		},
	}

	_, err := n.SendNextDayDigestForGroupSubgroup(context.Background(), schedProv, 101, 0)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if store.deactivatedChatID != 1002 {
		t.Errorf("expected chatID 1002 to be deactivated, got %d", store.deactivatedChatID)
	}
}

type countingFailBot struct {
	sendCalls int
}

func (m *countingFailBot) Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error) {
	m.sendCalls++
	return nil, tele.ErrBlockedByUser
}

type chunkTestStore struct {
	mockUserStore
}

func (m *chunkTestStore) UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error) {
	return []int64{2001}, nil
}

func TestNotifier_HandleGroupScheduleUpdate_BreaksChunkLoopOnError(t *testing.T) {
	bot := &countingFailBot{}
	store := &chunkTestStore{}
	n := New(slog.Default(), bot, store, nil, rate.NewLimiter(rate.Inf, 0))

	// Создаем diff с очень большим текстом, чтобы он разделился минимум на 2 чанка (>4096 символов)
	var pairs []entities.PairDiff
	for i := range 50 {
		pairs = append(pairs, entities.PairDiff{
			Number: i + 1,
			SubgroupDiffs: []entities.SubgroupDiff{
				{
					Subgroup: "1 п/г",
					Type:     entities.ChangeTypeModified,
					OldPair:  &entities.Pair{Name: "Длинное старое название дисциплины для деления на чанки", Class: "1-101", Group: "Преподаватель"},
					NewPair:  &entities.Pair{Name: "Длинное новое название дисциплины для деления на чанки", Class: "2-202", Group: "Другой преподаватель"},
				},
			},
		})
	}
	diff := &entities.ScheduleDiff{
		Title: "Группа 101",
		Days: []entities.DayDiff{
			{
				DayName: "Понедельник",
				Pairs:   pairs,
			},
		},
	}

	err := n.HandleGroupScheduleUpdate(context.Background(), &entities.Group{ID: 101}, diff)
	require.NoError(t, err)

	// Так как первый чанк вернул ошибку, бот должен прервать цикл чанков через break и сделать ровно 1 вызов
	assert.Equal(t, 1, bot.sendCalls, "должен быть отправлен только первый чанк, последующие должны быть отменены")
}

type userStoreSpy struct {
	mockUserStore

	calls int
}

func (s *userStoreSpy) UsersForEveningNotification(ctx context.Context) ([]entities.User, error) {
	s.calls++
	return s.mockUserStore.UsersForEveningNotification(ctx)
}

func TestNotifier_SendNextDayDigest_PreloadedUsers(t *testing.T) {
	bot := &mockBotSender{}
	store := &userStoreSpy{}
	n := New(slog.Default(), bot, store, nil, rate.NewLimiter(rate.Inf, 0))

	targetDate, _ := CalculateTomorrowDate(time.Now())
	var days []*entities.Day
	for i := range 6 {
		days = append(days, &entities.Day{
			Name: fmt.Sprintf("День %d", i),
			Subjects: []*entities.Subject{
				{Pairs: []*entities.Pair{{Name: "Информатика", Class: "101"}}},
			},
		})
	}
	group := &entities.Group{
		ID:   101,
		Week: targetDate,
		Days: days,
	}
	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return group, nil
		},
		latestFunc: func(ctx context.Context, groupID int32) (*entities.Group, error) {
			return group, nil
		},
	}

	preloaded := []entities.User{
		{ChatID: 1001, Group: 101, Subgroup: 0},
	}

	sentIDs, err := n.SendNextDayDigestForGroupSubgroup(context.Background(), schedProv, 101, 0, preloaded)
	require.NoError(t, err)
	assert.Equal(t, []int64{1001}, sentIDs)
	assert.Equal(t, 0, store.calls, "userStore.UsersForEveningNotification не должен вызываться при переданных preloadedUsers")
}
