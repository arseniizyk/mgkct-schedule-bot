package formatter

import (
	"errors"
	"strings"
	"testing"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
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
	day := &pb.Day{
		Name: "Пятница",
		Subjects: []*pb.Subject{
			{Pairs: []*pb.Pair{{Name: "Основы C* и C_", Type: "лб", Teacher: "Иванов_И. И.", Class: "3-205 (к)"}}},
		},
	}

	got := formatScheduleDay(day)

	if !strings.Contains(got, "Основы C\\* и C\\_") {
		t.Errorf("subject name should be escaped, got:\n%s", got)
	}
	if !strings.Contains(got, "Иванов\\_И. И.") {
		t.Errorf("teacher name should be escaped, got:\n%s", got)
	}
	if strings.Contains(got, "*Пятница\n*") == false {
		t.Errorf("day header should stay bold, got:\n%s", got)
	}
}

func TestFormatScheduleDaySecondShift(t *testing.T) {
	// группа второй смены: слоты 1-3 пустые, пары начинаются с №4
	day := &pb.Day{
		Name: "Понедельник",
		Subjects: []*pb.Subject{
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
			{Pairs: []*pb.Pair{{Name: "Математика", Type: "лк", Teacher: "Костян Д. Р.", Class: "1-322"}}},
			{IsEmpty: true},
			{IsEmpty: true},
			{Pairs: []*pb.Pair{{Name: "Физика", Type: "лк", Teacher: "Лебедкина Н. В.", Class: "2-314"}}},
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
	if !strings.Contains(got, "7: Физика | лк | Лебедкина Н. В. | 2-314") {
		t.Errorf("пара №7 должна иметь свой номер, got:\n%s", got)
	}
}

func TestFormatScheduleDayFirstShiftLeadingEmpty(t *testing.T) {
	// обычная группа: первый слот свободен — он информативный и должен показываться
	day := &pb.Day{
		Name: "Вторник",
		Subjects: []*pb.Subject{
			{IsEmpty: true},
			{Pairs: []*pb.Pair{{Name: "История", Type: "лк", Teacher: "Галенко Е. Л."}}},
		},
	}

	got := formatScheduleDay(day)

	if !strings.Contains(got, "*Вторник\n*") {
		t.Errorf("маркера смены быть не должно, got:\n%s", got)
	}
	if !strings.Contains(got, "1: ──\n2: История") {
		t.Errorf("свободный слот первой смены должен отображаться, got:\n%s", got)
	}
}

func TestFindLastSubject(t *testing.T) {
	tests := []struct {
		name     string
		subjects []*pb.Subject
		want     int
	}{
		{
			name:     "empty slice",
			subjects: nil,
			want:     -1,
		},
		{
			name:     "all empty",
			subjects: []*pb.Subject{{IsEmpty: true}, {IsEmpty: true}},
			want:     -1,
		},
		{
			name:     "last is filled",
			subjects: []*pb.Subject{{IsEmpty: true}, {Pairs: []*pb.Pair{{Name: "Математика"}}}},
			want:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findLastSubject(tt.subjects); got != tt.want {
				t.Errorf("findLastSubject() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatScheduleDay(t *testing.T) {
	t.Run("day off", func(t *testing.T) {
		day := &pb.Day{Name: "Понедельник", Subjects: []*pb.Subject{{IsEmpty: true}}}

		got := formatScheduleDay(day)
		if !strings.Contains(got, "*Понедельник") {
			t.Errorf("formatScheduleDay() missing day name header, got %q", got)
		}
		if !strings.Contains(got, "*Выходной*") {
			t.Errorf("formatScheduleDay() missing day-off marker, got %q", got)
		}
	})

	t.Run("single pair without class", func(t *testing.T) {
		day := &pb.Day{
			Name: "Вторник",
			Subjects: []*pb.Subject{
				{Pairs: []*pb.Pair{{Name: "Физика", Type: "лк", Teacher: "Иванов"}}},
			},
		}

		got := formatScheduleDay(day)
		want := "*Вторник\n*1: Физика | лк | Иванов | \n\n"

		if got != want {
			t.Errorf("formatScheduleDay() = %q, want %q", got, want)
		}
	})

	t.Run("single pair with class", func(t *testing.T) {
		day := &pb.Day{
			Name: "Вторник",
			Subjects: []*pb.Subject{
				{Pairs: []*pb.Pair{{Name: "Физика", Type: "пр", Teacher: "Иванов", Class: "301"}}},
			},
		}

		got := formatScheduleDay(day)
		want := "*Вторник\n*1: Физика | пр | Иванов | 301\n\n"

		if got != want {
			t.Errorf("formatScheduleDay() = %q, want %q", got, want)
		}
	})

	t.Run("multi pairs tree formatting and skipped empty before last", func(t *testing.T) {
		day := &pb.Day{
			Name: "Среда",
			Subjects: []*pb.Subject{
				{IsEmpty: true},
				{Pairs: []*pb.Pair{
					{Name: "Химия 1", Type: "лк", Teacher: "Петрова"},
					{Name: "Химия 2", Type: "пр", Teacher: "Петрова"},
				}},
			},
		}

		got := formatScheduleDay(day)
		want := "*Среда\n*" +
			"1: ──\n" +
			"2:\n" +
			"├─ Химия 1 | лк | Петрова | \n" +
			"└─ Химия 2 | пр | Петрова | \n" +
			"\n"

		if got != want {
			t.Errorf("formatScheduleDay() = %q, want %q", got, want)
		}
	})
}

func TestFormatScheduleWeek(t *testing.T) {
	group := &pb.Group{
		Days: []*pb.Day{
			{Name: "Пн", Subjects: []*pb.Subject{{IsEmpty: true}}},
			{Name: "Вт", Subjects: []*pb.Subject{{Pairs: []*pb.Pair{{Name: "Физика"}}}}},
		},
	}

	got := FormatScheduleWeek(group)

	for _, part := range []string{"*Пн", "*Выходной*", "*Вт", "1: Физика |"} {
		if !strings.Contains(got, part) {
			t.Errorf("FormatScheduleWeek() missing %q in result:\n%s", part, got)
		}
	}
}

func TestFormatSubjectsTrailingEmptiesCut(t *testing.T) {
	day := &pb.Day{
		Name: "Четверг",
		Subjects: []*pb.Subject{
			{Pairs: []*pb.Pair{{Name: "История"}}},
			{IsEmpty: true},
			{IsEmpty: true},
		},
	}

	got := formatScheduleDay(day)

	if strings.Contains(got, "2:") || strings.Contains(got, "3:") {
		t.Errorf("trailing empty subjects should be cut, got:\n%s", got)
	}
}
