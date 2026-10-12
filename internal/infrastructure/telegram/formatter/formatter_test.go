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
