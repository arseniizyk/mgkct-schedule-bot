package formatter

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

// ... [rest of the file remains as in the original state before my edits or as needed]

func TestFormatErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "user no group", err: domainerr.ErrUserNoGroup, want: messages.UserNoGroup},
		{name: "group not found", err: domainerr.ErrGroupNotFound, want: messages.GroupNotFound},
		{name: "schedule not found", err: domainerr.ErrScheduleNotFound, want: messages.GroupNotFound},
		{name: "internal", err: domainerr.ErrServiceInternal, want: messages.Internal},
		{name: "unknown error defaults to internal", err: errors.New("boom"), want: messages.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatErrorMessage(tt.err); got != tt.want {
				t.Errorf("FormatErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapeMD(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "underscore in teacher name", in: "Иванов_И", want: "Иванов\\_И"},
		{name: "asterisk in subject", in: "C* язык", want: "C\\* язык"},
		{name: "bracket", in: "[каб 5]", want: "\\[каб 5]"},
		{name: "clean text untouched", in: "Математика(Лек)Петрова", want: "Математика(Лек)Петрова"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeMD(tt.in); got != tt.want {
				t.Errorf("escapeMD(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatSubjectsEscapesDynamicFields(t *testing.T) {
	day := &entities.Day{
		Name: "Пятница",
		Subjects: []*entities.Subject{
			{Pairs: []*entities.Pair{{Name: "Основы C* и C_", Type: "лб", Group: "Иванов_И. И.", Class: "3-205 (к)"}}},
		},
	}

	got := formatScheduleDay(day)

	if !strings.Contains(got, "Основы C\\* и C\\_") {
		t.Errorf("subject name should be escaped, got:\n%s", got)
	}
	if !strings.Contains(got, "Иванов\\_И. И.") {
		t.Errorf("teacher name should be escaped, got:\n%s", got)
	}
}

func TestFormatScheduleDaySecondShift(t *testing.T) {
	day := &entities.Day{
		Name: "Понедельник",
		Subjects: []*entities.Subject{
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
			{Pairs: []*entities.Pair{{Name: "Математика", Type: "лк", Group: "Костян Д. Р.", Class: "1-322"}}},
			{IsEmpty: true},
			{IsEmpty: true},
			{Pairs: []*entities.Pair{{Name: "Физика", Type: "лк", Group: "Лебедкина Н. В.", Class: "2-314"}}},
		},
	}

	got := formatScheduleDay(day)

	if !strings.Contains(got, "*Понедельник (2 смена") {
		t.Errorf("день второй смены должен иметь маркер в заголовке, got:\n%s", got)
	}
	if strings.Contains(got, "1: ──") || strings.Contains(got, "2: ──") || strings.Contains(got, "3: ──") {
		t.Errorf("чужие слоты первой смены не должны отображаться, got:\n%s", got)
	}
	if !strings.Contains(got, "4: Математика | лк | Костян Д. Р. | 1-322") {
		t.Errorf("нумерация пар должна идти по номерам сайта, got:\n%s", got)
	}
}

func TestFormatScheduleDayAt(t *testing.T) {
	group := &entities.Group{
		Days: []*entities.Day{
			{Name: "Пн", Subjects: []*entities.Subject{{IsEmpty: true}}},
			{Name: "Вт", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Физика"}}}}},
		},
	}

	t.Run("valid index", func(t *testing.T) {
		got, err := FormatScheduleDayAt(group, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "*Вт") || !strings.Contains(got, "1: Физика") {
			t.Errorf("FormatScheduleDayAt(1) = %q", got)
		}
	})

	t.Run("out of range", func(t *testing.T) {
		if _, err := FormatScheduleDayAt(group, -1); err == nil {
			t.Error("expected error for negative index")
		}
		if _, err := FormatScheduleDayAt(group, 2); err == nil {
			t.Error("expected error for out-of-range index")
		}
	})
}

func TestFormatTeacherProfile(t *testing.T) {
	if got := FormatTeacherProfile(nil); got != "" {
		t.Errorf("expected empty string for nil member, got %q", got)
	}

	member := &entities.FacultyMember{
		FullName:      "Анципов Евгений Юрьевич",
		ShortName:     "Анципов Е. Ю.",
		Position:      "Преподаватель",
		Qualification: "Второй категории",
	}

	got := FormatTeacherProfile(member)
	if !strings.Contains(got, "Анципов Евгений Юрьевич") || !strings.Contains(got, "Анципов Е. Ю.") {
		t.Errorf("expected names in profile, got: %s", got)
	}
	if !strings.Contains(got, "Преподаватель • Второй категории") {
		t.Errorf("expected position and qualification, got: %s", got)
	}
}

func TestFormatCabinetDay(t *testing.T) {
	sched := &entities.CabinetSchedule{
		Cabinet: "301",
		Days: []*entities.CabinetDaySchedule{
			{
				DayIndex: 0,
				DayName:  "Понедельник",
				Bookings: []*entities.CabinetBooking{
					{
						DayIndex:  0,
						DayName:   "Понедельник",
						PairIndex: 1,
						GroupID:   101,
						Subject:   "Математика",
						Teacher:   "Иванов И.И.",
						Class:     "301",
					},
				},
			},
		},
	}

	got := FormatCabinetDay(sched, 0)
	if !strings.Contains(got, "Кабинет 301") {
		t.Errorf("expected cabinet 301, got %s", got)
	}
	if !strings.Contains(got, "Понедельник") {
		t.Errorf("expected day name, got %s", got)
	}
	if !strings.Contains(got, "1 пара: гр. *101* | Математика | Иванов И.И.") {
		t.Errorf("expected booking info, got %s", got)
	}

	weekGot := FormatCabinetWeek(sched)
	if !strings.Contains(weekGot, "Расписание кабинета 301 на неделю") {
		t.Errorf("expected week header, got %s", weekGot)
	}
}

func TestFormatEndings(t *testing.T) {
	endings := &entities.EndingsStat{
		DayIndex: 0,
		DayName:  "Понедельник",
		Pairs: map[int][]entities.GroupEnding{
			3: {{GroupID: 102, Campus: entities.CampusKnorina}},
			4: {{GroupID: 101, Subgroup: 1, Campus: entities.CampusKazintsa}},
		},
		DayOff:                 []entities.GroupEnding{{GroupID: 103, Campus: entities.CampusKazintsa}},
		Total:                  3,
		TotalStudents:          61,
		StudentsByPair:         map[int]int{3: 25, 4: 11},
		DayOffStudents:         25,
		KazintsaTotalStudents:  11,
		KnorinaTotalStudents:   25,
		KazintsaStudentsByPair: map[int]int{4: 11},
		KnorinaStudentsByPair:  map[int]int{3: 25},
	}

	got := FormatEndings(endings)
	if !strings.Contains(got, "Окончание пар: Понедельник") {
		t.Errorf("expected title with day, got %s", got)
	}
	if strings.Contains(got, "Всего в расписании") || strings.Contains(got, "*Казинца:*") || strings.Contains(got, "*Кнорина:*") {
		t.Errorf("expected no total or campus student summary in header, got %s", got)
	}
	if strings.Contains(got, "Корпус Казинца (") || strings.Contains(got, "Корпус Кнорина (") {
		t.Errorf("expected no total students in campus header, got %s", got)
	}
	if !strings.Contains(got, "Корпус Казинца") || !strings.Contains(got, "Корпус Кнорина") {
		t.Errorf("expected campus sections in message, got %s", got)
	}
	if !strings.Contains(got, "*После 3 пары* — 1 гр. (25 чел.):") || !strings.Contains(got, "*После 4 пары* — 1 гр. (11 чел.):") {
		t.Errorf("expected pairs and students in message, got %s", got)
	}
	if !strings.Contains(got, "101 (1 п/г)") {
		t.Errorf("expected subgroup 101 (1 п/г) in message, got %s", got)
	}
	if strings.Contains(got, "до ") || strings.Contains(got, "перерыв") {
		t.Errorf("expected no times in message, got %s", got)
	}
	if !strings.Contains(got, "Выходной") || !strings.Contains(got, "103") {
		t.Errorf("expected day off info, got %s", got)
	}
}

func TestFormatStats(t *testing.T) {
	stats := []entities.SubjectStat{
		{
			Subject:     "Математика",
			SubjectFull: "Высшая математика",
			Count:       8,
			Types:       map[string]int{"лк": 4, "пз": 4},
		},
	}

	got := FormatStats("Группа 101", stats, 16)
	if !strings.Contains(got, "Статистика нагрузки: Группа 101") {
		t.Errorf("expected header, got %s", got)
	}
	if !strings.Contains(got, "Всего пар на неделю: 16") {
		t.Errorf("expected totals, got %s", got)
	}
	if !strings.Contains(got, "Математика") || !strings.Contains(got, "8 пар (50%)") {
		t.Errorf("expected subject stat, got %s", got)
	}
	if !strings.Contains(got, "Высшая математика") {
		t.Errorf("expected subject full name, got %s", got)
	}
}

func TestFindLastSubject(t *testing.T) {
	tests := []struct {
		name     string
		subjects []*entities.Subject
		want     int
	}{
		{
			name:     "empty subjects",
			subjects: []*entities.Subject{},
			want:     -1,
		},
		{
			name: "all subjects with pairs empty",
			subjects: []*entities.Subject{
				{Pairs: []*entities.Pair{}},
				{Pairs: []*entities.Pair{{Name: "", Type: "", Group: "", Class: ""}}},
			},
			want: -1,
		},
		{
			name: "single subject with valid pair",
			subjects: []*entities.Subject{
				{Pairs: []*entities.Pair{{Name: "Математика"}}},
			},
			want: 0,
		},
		{
			name: "multiple subjects, last non-empty is index 2",
			subjects: []*entities.Subject{
				{Pairs: []*entities.Pair{{Name: "Физика"}}},
				{Pairs: []*entities.Pair{{Name: "Химия"}}},
				{Pairs: []*entities.Pair{{Name: "Информатика"}}},
				{Pairs: []*entities.Pair{}},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FindLastSubject(tt.subjects); got != tt.want {
				t.Errorf("FindLastSubject() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetDayEndTime(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)

	tests := []struct {
		name        string
		dayIdx      int
		lastSubject int
		wantHour    int
		wantMin     int
		wantOK      bool
	}{
		{
			name:        "weekday pair 1",
			dayIdx:      0, // Monday
			lastSubject: 1,
			wantHour:    10,
			wantMin:     40,
			wantOK:      true,
		},
		{
			name:        "weekday pair 3",
			dayIdx:      4, // Friday
			lastSubject: 3,
			wantHour:    14,
			wantMin:     40,
			wantOK:      true,
		},
		{
			name:        "Saturday pair 3",
			dayIdx:      5, // Saturday
			lastSubject: 3,
			wantHour:    14,
			wantMin:     30,
			wantOK:      true,
		},
		{
			name:        "invalid pair number 0",
			dayIdx:      0,
			lastSubject: 0,
			wantOK:      false,
		},
		{
			name:        "invalid pair number 7",
			dayIdx:      0,
			lastSubject: 7,
			wantOK:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotOK := GetDayEndTime(tt.dayIdx, tt.lastSubject, minskLoc)
			if gotOK != tt.wantOK {
				t.Fatalf("GetDayEndTime() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if tt.wantOK {
				if gotTime.Hour() != tt.wantHour || gotTime.Minute() != tt.wantMin {
					t.Errorf("GetDayEndTime() = %02d:%02d, want %02d:%02d",
						gotTime.Hour(), gotTime.Minute(), tt.wantHour, tt.wantMin)
				}
			}
		})
	}
}

func TestFormatGroupScheduleDiff(t *testing.T) {
	diff := &entities.ScheduleDiff{
		Title: "Группа 101",
		Days: []entities.DayDiff{
			{
				DayName:  "Вторник",
				DayIndex: 1,
				Date:     time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
				Pairs: []entities.PairDiff{
					{
						Number: 2,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Subgroup: "2 п/г",
								Type:     entities.ChangeTypeModified,
								OldPair:  &entities.Pair{Name: "Информатика", Class: "305"},
								NewPair:  &entities.Pair{Name: "Информатика", Class: "102"},
							},
						},
					},
					{
						Number: 3,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Subgroup: "",
								Type:     entities.ChangeTypeAdded,
								NewPair:  &entities.Pair{Name: "Информатика", Type: "Лаб", Class: "102"},
							},
						},
					},
					{
						Number: 4,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Subgroup: "",
								Type:     entities.ChangeTypeRemoved,
								OldPair:  &entities.Pair{Name: "Химия", Class: "110"},
							},
						},
					},
				},
			},
		},
	}

	got := FormatGroupScheduleDiff(diff)
	expected := "🔔 <b>Изменения в расписании</b>\n" +
		"📅 <b>Вторник, 15 октября</b>\n\n" +
		"▫️ <b>2 пара • 2 п/г</b>\n" +
		"  🔄 Информатика: ауд. <s>305</s> ➔ <b>102</b>\n\n" +
		"▫️ <b>3 пара</b>\n" +
		"  🆕 <i>Новая пара:</i> <b>Информатика • Лаб • ауд. 102</b>\n\n" +
		"▫️ <b>4 пара</b>\n" +
		"  ❌ <i>Отменена:</i> <s>Химия • ауд. 110</s>"

	assert.Equal(t, expected, got)
}

func TestFormatTeacherScheduleDiff(t *testing.T) {
	diff := &entities.ScheduleDiff{
		Title: "Иванов И.И.",
		Days: []entities.DayDiff{
			{
				DayName:  "Среда",
				DayIndex: 2,
				Date:     time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC),
				Pairs: []entities.PairDiff{
					{
						Number: 1,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Subgroup: "",
								Type:     entities.ChangeTypeModified,
								OldPair:  &entities.Pair{Group: "101", Name: "Математика", Class: "204"},
								NewPair:  &entities.Pair{Group: "102", Name: "Математика", Class: "301"},
							},
						},
					},
					{
						Number: 3,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Subgroup: "",
								Type:     entities.ChangeTypeRemoved,
								OldPair:  &entities.Pair{Group: "201", Name: "Высшая математика", Class: "105"},
							},
						},
					},
				},
			},
		},
	}

	got := FormatTeacherScheduleDiff(diff)
	expected := "🔔 <b>Изменения в расписании</b>\n" +
		"📅 <b>Среда, 16 октября</b>\n\n" +
		"▫️ <b>1 пара</b>\n" +
		"  <s>101 • Математика • ауд. 204</s>\n" +
		"  👉 <b>102 • Математика • ауд. 301</b>\n\n" +
		"▫️ <b>3 пара</b>\n" +
		"  ❌ <i>Отменена:</i> <s>201 • Высшая математика • ауд. 105</s>"

	assert.Equal(t, expected, got)
}

func TestFormatScheduleDiff_HTMLEscaping(t *testing.T) {
	diff := &entities.ScheduleDiff{
		Title: "Группа <101> & Co",
		Days: []entities.DayDiff{
			{
				DayName:  "Пятница",
				DayIndex: 4,
				Date:     time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC),
				Pairs: []entities.PairDiff{
					{
						Number: 1,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Subgroup: "1 <п/г>",
								Type:     entities.ChangeTypeAdded,
								NewPair:  &entities.Pair{Name: "C++ & C#", Type: "<Лаб>", Class: "201 <к>"},
							},
						},
					},
				},
			},
		},
	}

	got := FormatGroupScheduleDiff(diff)
	assert.Contains(t, got, "1 &lt;п/г&gt;")
	assert.Contains(t, got, "C++ &amp; C#")
	assert.Contains(t, got, "&lt;Лаб&gt;")
	assert.Contains(t, got, "201 &lt;к&gt;")
	assert.NotContains(t, got, "<101>")
	assert.NotContains(t, got, "<Лаб>")
}

func TestSplitMessage_WithDiffHTML(t *testing.T) {
	diff := &entities.ScheduleDiff{
		Title: "Группа 101",
		Days: []entities.DayDiff{
			{
				DayName:  "Понедельник",
				DayIndex: 0,
				Date:     time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
				Pairs: []entities.PairDiff{
					{
						Number: 1,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Type:    entities.ChangeTypeAdded,
								NewPair: &entities.Pair{Name: "Предмет 1", Class: "101"},
							},
						},
					},
					{
						Number: 2,
						SubgroupDiffs: []entities.SubgroupDiff{
							{
								Type:    entities.ChangeTypeAdded,
								NewPair: &entities.Pair{Name: "Предмет 2", Class: "102"},
							},
						},
					},
				},
			},
		},
	}

	formatted := FormatGroupScheduleDiff(diff)
	chunks := SplitMessage(formatted, 60)
	assert.Greater(t, len(chunks), 1)

	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk)
		assert.LessOrEqual(t, len([]rune(chunk)), 60)
	}
}

func TestParsePairSubgroup(t *testing.T) {
	tests := []struct {
		name string
		pair *entities.Pair
		want int
	}{
		{name: "nil pair", pair: nil, want: 0},
		{name: "subgroup 1 prefix dot", pair: &entities.Pair{Name: "1. Информатика"}, want: 1},
		{name: "subgroup 2 prefix dot", pair: &entities.Pair{Name: "2. Информатика"}, want: 2},
		{name: "subgroup 3 prefix dot", pair: &entities.Pair{Name: "3. Немецкий язык"}, want: 3},
		{name: "subgroup 1 in group field", pair: &entities.Pair{Name: "Физика", Group: "1 п/г"}, want: 1},
		{name: "subgroup 2 in group field", pair: &entities.Pair{Name: "Физика", Group: "2 п/гр"}, want: 2},
		{name: "subgroup 3 in group field", pair: &entities.Pair{Name: "Физика", Group: "3 п/г"}, want: 3},
		{name: "subgroup single digit group field", pair: &entities.Pair{Name: "Химия", Group: "1"}, want: 1},
		{name: "general pair", pair: &entities.Pair{Name: "Математика", Group: "Иванов И. И."}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePairSubgroup(tt.pair)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterGroupForSubgroup_TwoSubgroups(t *testing.T) {
	group := &entities.Group{
		ID: 101,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						// Пара 1: деление на 2 подгруппы
						Pairs: []*entities.Pair{
							{Name: "1. Программирование", Type: "лб", Class: "305"},
							{Name: "2. Электротехника", Type: "лб", Class: "102"},
						},
					},
					{
						// Пара 2: общая пара
						Pairs: []*entities.Pair{
							{Name: "Высшая математика", Type: "лк", Class: "301"},
						},
					},
				},
			},
		},
	}

	t.Run("subgroup 1 receives only its pair", func(t *testing.T) {
		filtered := FilterGroupForSubgroup(group, 1)
		day := filtered.Days[0]
		require.Len(t, day.Subjects, 2)

		// Пара 1 должна содержать только пару 1-й подгруппы
		require.False(t, day.Subjects[0].IsEmpty)
		require.Len(t, day.Subjects[0].Pairs, 1)
		assert.Equal(t, "1. Программирование", day.Subjects[0].Pairs[0].Name)

		// Пара 2 общая — должна остаться
		require.False(t, day.Subjects[1].IsEmpty)
		require.Len(t, day.Subjects[1].Pairs, 1)
		assert.Equal(t, "Высшая математика", day.Subjects[1].Pairs[0].Name)

		msg := FormatScheduleDayForSubgroup(group, 1)
		assert.Contains(t, msg, "1. Программирование")
		assert.NotContains(t, msg, "2. Электротехника")
		assert.Contains(t, msg, "Высшая математика")
	})

	t.Run("subgroup 2 receives only its pair", func(t *testing.T) {
		filtered := FilterGroupForSubgroup(group, 2)
		day := filtered.Days[0]
		require.Len(t, day.Subjects, 2)

		// Пара 1 должна содержать только пару 2-й подгруппы
		require.False(t, day.Subjects[0].IsEmpty)
		require.Len(t, day.Subjects[0].Pairs, 1)
		assert.Equal(t, "2. Электротехника", day.Subjects[0].Pairs[0].Name)

		// Пара 2 общая — должна остаться
		require.False(t, day.Subjects[1].IsEmpty)
		require.Len(t, day.Subjects[1].Pairs, 1)
		assert.Equal(t, "Высшая математика", day.Subjects[1].Pairs[0].Name)

		msg := FormatScheduleDayForSubgroup(group, 2)
		assert.NotContains(t, msg, "1. Программирование")
		assert.Contains(t, msg, "2. Электротехника")
		assert.Contains(t, msg, "Высшая математика")
	})

	t.Run("subgroup 0 receives all pairs", func(t *testing.T) {
		filtered := FilterGroupForSubgroup(group, 0)
		day := filtered.Days[0]
		require.Len(t, day.Subjects[0].Pairs, 2)

		msg := FormatScheduleDayForSubgroup(group, 0)
		assert.Contains(t, msg, "1. Программирование")
		assert.Contains(t, msg, "2. Электротехника")
		assert.Contains(t, msg, "Высшая математика")
	})
}

func TestFilterGroupForSubgroup_ThreeSubgroupsException(t *testing.T) {
	group := &entities.Group{
		ID: 101,
		Days: []*entities.Day{
			{
				Name: "Вторник",
				Subjects: []*entities.Subject{
					{
						// Пара с 3 подгруппами (иностранные языки / спецпредметы)
						Pairs: []*entities.Pair{
							{Name: "1. Английский язык", Type: "пз", Class: "201"},
							{Name: "2. Немецкий язык", Type: "пз", Class: "202"},
							{Name: "3. Французский язык", Type: "пз", Class: "203"},
						},
					},
				},
			},
		},
	}

	// Для пар с 3 подгруппами исключение: выводить расписание для всех!
	for _, sg := range []int{1, 2} {
		t.Run(fmt.Sprintf("subgroup %d sees all 3 subgroups", sg), func(t *testing.T) {
			filtered := FilterGroupForSubgroup(group, sg)
			require.Len(t, filtered.Days[0].Subjects[0].Pairs, 3)

			got, err := FormatScheduleDayAtForSubgroup(group, 0, sg)
			require.NoError(t, err)
			assert.Contains(t, got, "1. Английский язык")
			assert.Contains(t, got, "2. Немецкий язык")
			assert.Contains(t, got, "3. Французский язык")
		})
	}
}

func TestFilterGroupForSubgroup_SingleSubgroupInSlot(t *testing.T) {
	group := &entities.Group{
		ID: 101,
		Days: []*entities.Day{
			{
				Name: "Среда",
				Subjects: []*entities.Subject{
					{
						// 1 пара: только у 1-й подгруппы
						Pairs: []*entities.Pair{
							{Name: "1. Лабораторная", Type: "лб", Class: "305"},
						},
					},
					{
						// 2 пара: только у 2-й подгруппы
						Pairs: []*entities.Pair{
							{Name: "2. Лабораторная", Type: "лб", Class: "305"},
						},
					},
				},
			},
		},
	}

	t.Run("subgroup 1 has pair 1 and window on pair 2", func(t *testing.T) {
		filtered := FilterGroupForSubgroup(group, 1)
		day := filtered.Days[0]

		require.False(t, day.Subjects[0].IsEmpty)
		assert.Equal(t, "1. Лабораторная", day.Subjects[0].Pairs[0].Name)

		// У 1-й подгруппы во 2-й паре окно
		assert.True(t, day.Subjects[1].IsEmpty)

		msg, err := FormatScheduleDayAtForSubgroup(group, 0, 1)
		require.NoError(t, err)
		assert.Contains(t, msg, "1. Лабораторная")
		assert.NotContains(t, msg, "2. Лабораторная")
	})

	t.Run("subgroup 2 has window on pair 1 and pair 2", func(t *testing.T) {
		filtered := FilterGroupForSubgroup(group, 2)
		day := filtered.Days[0]

		// У 2-й подгруппы в 1-й паре окно
		assert.True(t, day.Subjects[0].IsEmpty)

		require.False(t, day.Subjects[1].IsEmpty)
		assert.Equal(t, "2. Лабораторная", day.Subjects[1].Pairs[0].Name)

		msg, err := FormatScheduleDayAtForSubgroup(group, 0, 2)
		require.NoError(t, err)
		assert.NotContains(t, msg, "1. Лабораторная")
		assert.Contains(t, msg, "2. Лабораторная")
	})
}
func TestIsCurrentWeek(t *testing.T) {
	currentMonday := time.Date(2026, 10, 26, 0, 0, 0, 0, minskLoc)

	tests := []struct {
		name string
		week time.Time
		want bool
	}{
		{
			name: "parsed week in UTC matches current monday in MSK",
			week: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "week in MSK matches current monday",
			week: time.Date(2026, 10, 26, 0, 0, 0, 0, minskLoc),
			want: true,
		},
		{
			name: "next week in UTC does not match",
			week: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "past week in UTC does not match",
			week: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "zero week matches",
			week: time.Time{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCurrentWeek(tt.week, currentMonday)
			assert.Equal(t, tt.want, got)
		})
	}
}
