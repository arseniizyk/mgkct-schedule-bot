package formatter

import (
	"errors"
	"strings"
	"testing"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

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
		Pairs: map[int][]int32{
			3: {102},
			4: {101},
		},
		DayOff: []int32{103},
		Total:  3,
	}

	got := FormatEndings(endings)
	if !strings.Contains(got, "Окончание пар: Понедельник") {
		t.Errorf("expected title with day, got %s", got)
	}
	if !strings.Contains(got, "После 3 пары") || !strings.Contains(got, "После 4 пары") {
		t.Errorf("expected pairs in message, got %s", got)
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
