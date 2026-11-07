package widget_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	httpwidget "github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/widget"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

type mockScheduleService struct {
	group *entities.Group
}

func (m *mockScheduleService) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	return m.group, nil
}

type mockTeacherService struct {
	teacher *entities.Teacher
}

func (m *mockTeacherService) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	return m.teacher, nil
}

func (m *mockTeacherService) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	if m.teacher == nil {
		return "", nil, false
	}
	return m.teacher.Name, nil, true
}

func TestPreviewHandler_Group(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	group := &entities.Group{
		ID:   81,
		Week: time.Date(2026, 11, 9, 0, 0, 0, 0, timeutil.MinskLocation),
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Веб-разработка", Class: "каб. 214"},
						},
					},
				},
			},
		},
	}

	handler := httpwidget.NewHandler(logger, &mockScheduleService{group: group}, &mockTeacherService{})

	req := httptest.NewRequest(http.MethodGet, "/widget/preview?group=81", nil)
	rec := httptest.NewRecorder()

	handler.Preview(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))

	bodyBytes, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	body := string(bodyBytes)
	assert.Contains(t, body, "Группа 81")
	assert.Contains(t, body, "Веб-разработка")
	assert.Contains(t, body, "schedView")
}

func TestPreviewHandler_Teacher(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	teacher := &entities.Teacher{
		Name: "Иванов И.И.",
		Week: time.Date(2026, 11, 9, 0, 0, 0, 0, timeutil.MinskLocation),
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Философия", Class: "каб. 101", Group: "81"},
						},
					},
				},
			},
		},
	}

	handler := httpwidget.NewHandler(logger, &mockScheduleService{}, &mockTeacherService{teacher: teacher})

	req := httptest.NewRequest(http.MethodGet, "/widget/preview?teacher=Иванов", nil)
	rec := httptest.NewRecorder()

	handler.Preview(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	bodyBytes, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	body := string(bodyBytes)
	assert.Contains(t, body, "Иванов И.И.")
	assert.Contains(t, body, "Философия")
}

func TestPreviewHandler_DefaultDemo(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	handler := httpwidget.NewHandler(logger, &mockScheduleService{}, &mockTeacherService{})

	req := httptest.NewRequest(http.MethodGet, "/widget/preview", nil)
	rec := httptest.NewRecorder()

	handler.Preview(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	bodyBytes, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	body := string(bodyBytes)
	assert.Contains(t, body, "Группа 81")
	assert.Contains(t, body, "Основы алгоритмизации")
}
