package widget_test

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/widget"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

func mockWeeklyGroupSchedule() *entities.Group {
	week := time.Date(2026, 11, 9, 0, 0, 0, 0, timeutil.MinskLocation)
	days := make([]*entities.Day, 6)
	dayNames := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

	for i := range 6 {
		subjects := make([]*entities.Subject, 5)
		for s := range 5 {
			switch {
			case s == 3 && i%2 == 0:
				// Две подгруппы
				subjects[s] = &entities.Subject{
					Pairs: []*entities.Pair{
						{Name: "1. Программирование", Type: "ЛБ", Class: "каб. 214", Group: "1 п/г"},
						{Name: "2. Базы данных", Type: "ПР", Class: "каб. 302", Group: "2 п/г"},
					},
				}
			case s == 4 && i == 5:
				// Пустая пара
				subjects[s] = &entities.Subject{IsEmpty: true}
			default:
				subjects[s] = &entities.Subject{
					Pairs: []*entities.Pair{
						{Name: "Математика и алгоритмы", Type: "ЛК", Class: "каб. 101", Group: "Иванов И.И."},
					},
				}
			}
		}
		days[i] = &entities.Day{
			Name:     dayNames[i],
			Subjects: subjects,
		}
	}

	return &entities.Group{
		ID:   81,
		Week: week,
		Days: days,
	}
}

func mockWeeklyTeacherSchedule() *entities.Teacher {
	week := time.Date(2026, 11, 9, 0, 0, 0, 0, timeutil.MinskLocation)
	days := make([]*entities.Day, 6)
	dayNames := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

	for i := range 6 {
		subjects := make([]*entities.Subject, 4)
		for s := range 4 {
			subjects[s] = &entities.Subject{
				Pairs: []*entities.Pair{
					{Name: "Операционные системы", Type: "ЛК", Class: "каб. 204", Group: "81"},
				},
			}
		}
		days[i] = &entities.Day{
			Name:     dayNames[i],
			Subjects: subjects,
		}
	}

	return &entities.Teacher{
		Name: "Сидоров С.С.",
		Week: week,
		Days: days,
	}
}

func TestBuildGroupWidget_SizeConstraints(t *testing.T) {
	group := mockWeeklyGroupSchedule()
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, timeutil.MinskLocation)

	html, err := widget.BuildGroupWidget(group, now)
	require.NoError(t, err)
	require.NotEmpty(t, html)

	rawLen, compLen, err := widget.ValidateSize(html)
	require.NoError(t, err, "Widget must satisfy Telegram Widget Profile 1 size limits")

	t.Logf("Group Widget Size: raw=%d bytes (limit 32768), compressed=%d bytes (limit 4096)", rawLen, compLen)

	assert.LessOrEqual(t, rawLen, widget.MaxUncompressedSize)
	assert.LessOrEqual(t, compLen, widget.MaxCompressedSize)
	assert.Contains(t, html, "Группа 81")
	assert.Contains(t, html, "Математика и алгоритмы")
	assert.Contains(t, html, "214")
}

func TestBuildTeacherWidget_SizeConstraints(t *testing.T) {
	teacher := mockWeeklyTeacherSchedule()
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, timeutil.MinskLocation)

	html, err := widget.BuildTeacherWidget(teacher, now)
	require.NoError(t, err)
	require.NotEmpty(t, html)

	rawLen, compLen, err := widget.ValidateSize(html)
	require.NoError(t, err, "Widget must satisfy Telegram Widget Profile 1 size limits")

	t.Logf("Teacher Widget Size: raw=%d bytes (limit 32768), compressed=%d bytes (limit 4096)", rawLen, compLen)

	assert.LessOrEqual(t, rawLen, widget.MaxUncompressedSize)
	assert.LessOrEqual(t, compLen, widget.MaxCompressedSize)
	assert.Contains(t, html, "Сидоров С.С.")
	assert.Contains(t, html, "Операционные системы")
}

func TestBuildWidget_NilSafety(t *testing.T) {
	_, err := widget.BuildGroupWidget(nil)
	require.Error(t, err)

	_, err = widget.BuildTeacherWidget(nil)
	require.Error(t, err)
}

func TestCompress_Roundtrip(t *testing.T) {
	sample := "<html><body>Hello Telegram Widget Sandbox</body></html>"
	compressed, err := widget.Compress(sample)
	require.NoError(t, err)
	assert.NotEmpty(t, compressed)
}

func TestValidateSize_ExceedsLimits(t *testing.T) {
	hugeHTML := strings.Repeat("A", widget.MaxUncompressedSize+10)
	_, _, err := widget.ValidateSize(hugeHTML)
	require.Error(t, err)
}

func TestRealGroup88(t *testing.T) {
	dbPath := "../../../../bot.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Skipf("skipping db test: %v", err)
	}
	defer db.Close()

	var schedJSON string
	err = db.QueryRow("SELECT json_extract(schedule, '$.groups.88') FROM schedules WHERE week = '2026-11-02'").Scan(&schedJSON)
	if err != nil {
		t.Skipf("skipping: %v", err)
	}

	var group entities.Group
	err = json.Unmarshal([]byte(schedJSON), &group)
	require.NoError(t, err)

	now := time.Date(2026, 11, 7, 16, 5, 0, 0, timeutil.MinskLocation) // Saturday (today)
	html, err := widget.BuildGroupWidget(&group, now)
	require.NoError(t, err)

	dataJSON, _ := json.Marshal(group)
	compData, _ := widget.Compress(string(dataJSON))
	t.Logf("Group 88 original entity: raw=%d bytes, compressed=%d bytes", len(dataJSON), len(compData))

	rawLen, compLen, err := widget.ValidateSize(html)
	t.Logf("Group 88 size: raw=%d bytes, compressed=%d bytes (limit %d)", rawLen, compLen, widget.MaxCompressedSize)
	require.NoError(t, err)
}

func TestBuildGroupWidget_EmptyPairs(t *testing.T) {
	group := &entities.Group{
		ID:   88,
		Week: time.Date(2026, 11, 2, 0, 0, 0, 0, timeutil.MinskLocation),
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{IsEmpty: true}, // 1 пара пустая
					{IsEmpty: true}, // 2 пара пустая
					{
						Pairs: []*entities.Pair{
							{Name: "История", Type: "Лек", Class: "2-316", Group: "Тихонович Н. В."},
						},
					},
				},
			},
			{
				Name: "Вторник",
				Subjects: []*entities.Subject{
					{IsEmpty: true}, // Выходной день
				},
			},
		},
	}

	html, err := widget.BuildGroupWidget(group)
	require.NoError(t, err)
	assert.Contains(t, html, "empty-pair")
	assert.Contains(t, html, "История")
}
