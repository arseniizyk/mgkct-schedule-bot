package calendar

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

type mockDict struct {
	mapping map[string]string
}

func (m *mockDict) Resolve(raw string) (string, string, bool) {
	val, ok := m.mapping[raw]
	return raw, val, ok
}

func TestGenerateGroupCalendar(t *testing.T) {
	monday := time.Date(2026, 11, 9, 0, 0, 0, 0, minskLocation)

	group := &entities.Group{
		ID:   101,
		Week: monday,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{
								Name:  "Информатика",
								Type:  "Лаб",
								Group: "1 п/г",
								Class: "102",
							},
							{
								Name:  "Физика",
								Type:  "Пр",
								Group: "2 п/г",
								Class: "305",
							},
						},
					},
					{
						Pairs: []*entities.Pair{
							{
								Name:  "Математика",
								Type:  "Лек",
								Group: "",
								Class: "204",
							},
						},
					},
				},
			},
		},
	}

	t.Run("all subgroups (subgroup 0)", func(t *testing.T) {
		icsBytes, err := GenerateGroupCalendar(group, 0, nil)
		require.NoError(t, err)
		ics := UnfoldICS(string(icsBytes))

		assert.Contains(t, ics, "BEGIN:VCALENDAR")
		assert.Contains(t, ics, "X-WR-CALNAME:Группа 101")
		assert.Contains(t, ics, "BEGIN:VTIMEZONE")
		assert.Contains(t, ics, "TZID:Europe/Minsk")
		assert.Contains(t, ics, "SUMMARY:Информатика (Лаб)")
		assert.Contains(t, ics, "LOCATION:ауд. 102")
		assert.Contains(t, ics, "SUMMARY:Физика (Пр)")
		assert.Contains(t, ics, "LOCATION:ауд. 305")
		assert.Contains(t, ics, "SUMMARY:Математика (Лек)")
		assert.Contains(t, ics, "LOCATION:ауд. 204")
		assert.Contains(t, ics, "DTSTART;TZID=Europe/Minsk:20261109T090000")
		assert.Contains(t, ics, "DTEND;TZID=Europe/Minsk:20261109T104000")
		assert.Contains(t, ics, "DTSTART;TZID=Europe/Minsk:20261109T105000")
		assert.Contains(t, ics, "DTEND;TZID=Europe/Minsk:20261109T124000")
		assert.Contains(t, ics, "END:VCALENDAR")
	})

	t.Run("subgroup 1 filters out subgroup 2 pair", func(t *testing.T) {
		dict := &mockDict{
			mapping: map[string]string{
				"Информатика": "Основы информатики и программирования",
			},
		}

		icsBytes, err := GenerateGroupCalendar(group, 1, dict)
		require.NoError(t, err)
		ics := UnfoldICS(string(icsBytes))

		assert.Contains(t, ics, "X-WR-CALNAME:Группа 101 (1 п/г)")
		assert.Contains(t, ics, "SUMMARY:Основы информатики и программирования (Лаб)")
		assert.Contains(t, ics, "LOCATION:ауд. 102")
		// Физика (2 п/г) не должна попадать в календарь 1-й подгруппы
		assert.NotContains(t, ics, "Физика")
		// Общая пара Математика должна присутствовать
		assert.Contains(t, ics, "SUMMARY:Математика (Лек)")
	})

	t.Run("nil group error", func(t *testing.T) {
		_, err := GenerateGroupCalendar(nil, 0, nil)
		assert.Error(t, err)
	})
}

func TestGenerateTeacherCalendar(t *testing.T) {
	monday := time.Date(2026, 11, 9, 0, 0, 0, 0, minskLocation)

	teacher := &entities.Teacher{
		Name: "Иванов И. И.",
		Week: monday,
		Days: []*entities.Day{
			{
				Name: "Вторник",
				Subjects: []*entities.Subject{
					nil, // 1 пара окно
					{
						Pairs: []*entities.Pair{
							{
								Name:  "ТРПО",
								Type:  "Лаб",
								Group: "101",
								Class: "104",
							},
						},
					},
				},
			},
		},
	}

	icsBytes, err := GenerateTeacherCalendar(teacher, nil)
	require.NoError(t, err)
	ics := UnfoldICS(string(icsBytes))

	assert.Contains(t, ics, "BEGIN:VCALENDAR")
	assert.Contains(t, ics, "X-WR-CALNAME:Преподаватель Иванов И. И.")
	assert.Contains(t, ics, "SUMMARY:ТРПО (Лаб)")
	assert.Contains(t, ics, "LOCATION:ауд. 104")
	assert.Contains(t, ics, "DESCRIPTION:Группа: 101\\nПреподаватель: Иванов И. И.\\nПара: 2")
	// Вторник = monday + 1 день -> 20261110, пара 2 -> 10:50 - 12:40
	assert.Contains(t, ics, "DTSTART;TZID=Europe/Minsk:20261110T105000")
	assert.Contains(t, ics, "DTEND;TZID=Europe/Minsk:20261110T124000")
	assert.Contains(t, ics, "UID:teacher-20261110-p2-tperson-")
	assert.Contains(t, ics, "END:VCALENDAR")

	t.Run("distinct Cyrillic teachers have different UIDs", func(t *testing.T) {
		teacher2 := &entities.Teacher{
			Name: "Петров П. П.",
			Week: monday,
			Days: teacher.Days,
		}
		ics2Bytes, err := GenerateTeacherCalendar(teacher2, nil)
		require.NoError(t, err)
		ics2 := UnfoldICS(string(ics2Bytes))

		uid1 := extractUID(ics)
		uid2 := extractUID(ics2)
		assert.NotEmpty(t, uid1)
		assert.NotEmpty(t, uid2)
		assert.NotEqual(t, uid1, uid2, "teachers with different Cyrillic names must not collide in UID")
	})

	t.Run("nil teacher error", func(t *testing.T) {
		_, err := GenerateTeacherCalendar(nil, nil)
		assert.Error(t, err)
	})
}

func TestFoldingAndEscaping(t *testing.T) {
	longDescription := strings.Repeat("Очень длинное описание предмета и аудитории для проверки folding RFC 5545 ", 3)
	ev := event{
		UID:         "test-uid-123",
		Start:       time.Date(2026, 11, 9, 9, 0, 0, 0, minskLocation),
		End:         time.Date(2026, 11, 9, 10, 40, 0, 0, minskLocation),
		Summary:     "Предмет; с запятыми, и слэшами \\ тест",
		Location:    "ауд. 101",
		Description: longDescription,
	}

	rawICS := string(renderICS("Тест", []event{ev}))

	// Проверяем, что исходный текст корректно распаковывается через Unfold
	unfolded := UnfoldICS(rawICS)
	assert.Contains(t, unfolded, "SUMMARY:Предмет\\; с запятыми\\, и слэшами \\\\ тест")
	assert.Contains(t, unfolded, longDescription)

	// Проверяем каждую строку на лимит длины и валидность UTF-8
	for line := range strings.SplitSeq(rawICS, "\r\n") {
		if line == "" {
			continue
		}
		assert.LessOrEqual(t, len(line), 75, "line exceeds 75 octets: %s", line)
		assert.True(t, utf8.ValidString(line), "line contains broken UTF-8: %s", line)
	}
}

func TestDayOfWeekIndex(t *testing.T) {
	assert.Equal(t, 0, DayOfWeekIndex("Понедельник", -1))
	assert.Equal(t, 1, DayOfWeekIndex("Вторник", -1))
	assert.Equal(t, 2, DayOfWeekIndex("Среда", -1))
	assert.Equal(t, 3, DayOfWeekIndex("Четверг", -1))
	assert.Equal(t, 4, DayOfWeekIndex("Пятница", -1))
	assert.Equal(t, 5, DayOfWeekIndex("Суббота", -1))
	assert.Equal(t, 6, DayOfWeekIndex("Воскресенье", -1))
	assert.Equal(t, 99, DayOfWeekIndex("Неизвестный день", 99))
}

func extractUID(ics string) string {
	for line := range strings.SplitSeq(ics, "\r\n") {
		if uid, ok := strings.CutPrefix(line, "UID:"); ok {
			return uid
		}
	}
	return ""
}
