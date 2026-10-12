package keyboard

import (
	"strings"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

func btnFull(t *testing.T, row []tele.InlineButton) []string {
	t.Helper()
	res := make([]string, 0, len(row))
	for _, b := range row {
		res = append(res, b.Unique+"|"+b.Data)
	}
	return res
}

func TestInlineScheduleKeyboard(t *testing.T) {
	if got := InlineScheduleKeyboard(0); got != nil {
		t.Errorf("InlineScheduleKeyboard(0) = %v, want nil", got)
	}

	m := InlineScheduleKeyboard(99)
	if m == nil || len(m.InlineKeyboard) != 1 || len(m.InlineKeyboard[0]) != 1 {
		t.Fatalf("unexpected keyboard structure: %+v", m)
	}

	btn := m.InlineKeyboard[0][0]
	if btn.Unique+"|"+btn.Data != "currentweek|99" {
		t.Errorf("callback = %q, want %q", btn.Unique+"|"+btn.Data, "currentweek|99")
	}
	if !strings.Contains(btn.Text, "Неделя") {
		t.Errorf("btn.Text = %q, want week label", btn.Text)
	}
}

func TestInlineDayKeyboard(t *testing.T) {
	if got := InlineDayKeyboard(0, 2, 6); got != nil {
		t.Errorf("InlineDayKeyboard(0, ...) = %v, want nil", got)
	}

	t.Run("monday", func(t *testing.T) {
		m := InlineDayKeyboard(88, 0, 6)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:5", "currentweek|88", "nextday|88:1"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})

	t.Run("saturday wraps to monday", func(t *testing.T) {
		m := InlineDayKeyboard(88, 5, 6)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:4", "currentweek|88", "nextday|88:0"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})
}

func TestInlineWeekKeyboard(t *testing.T) {
	current := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)

	t.Run("no weeks available", func(t *testing.T) {
		m := InlineWeekKeyboard(99, entities.WeekNavigation{})
		if m == nil || len(m.InlineKeyboard) != 0 {
			t.Errorf("expected empty inline keyboard, got %+v", m)
		}
	})

	t.Run("both directions", func(t *testing.T) {
		weeks := entities.WeekNavigation{
			Prev:    current.AddDate(0, 0, -7),
			Current: current,
			Next:    current.AddDate(0, 0, 7),
		}

		m := InlineWeekKeyboard(88, weeks)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevweek|88:17.08.2026", "currentweek|88", "nextweek|88:31.08.2026"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})
}

func TestReplyScheduleKeyboard(t *testing.T) {
	if !ReplyScheduleKeyboard.ResizeKeyboard {
		t.Errorf("expected ReplyScheduleKeyboard.ResizeKeyboard = true, got false")
	}
	if ReplyScheduleKeyboard.IsPersistent {
		t.Errorf("expected ReplyScheduleKeyboard.IsPersistent = false to allow swipe gestures, got true")
	}

	markupWithoutApp := BuildReplyScheduleKeyboard("")
	if !markupWithoutApp.ResizeKeyboard {
		t.Errorf("expected BuildReplyScheduleKeyboard(\"\").ResizeKeyboard = true, got false")
	}
	if markupWithoutApp.IsPersistent {
		t.Errorf("expected BuildReplyScheduleKeyboard(\"\").IsPersistent = false, got true")
	}
	if len(markupWithoutApp.ReplyKeyboard) != 2 {
		t.Errorf("expected 2 rows without webApp, got %d", len(markupWithoutApp.ReplyKeyboard))
	}

	markupWithApp := BuildReplyScheduleKeyboard("https://example.com")
	if markupWithApp.IsPersistent {
		t.Errorf("expected BuildReplyScheduleKeyboard with app URL to have IsPersistent = false, got true")
	}
	if len(markupWithApp.ReplyKeyboard) != 3 {
		t.Errorf("expected 3 rows with webApp, got %d", len(markupWithApp.ReplyKeyboard))
	}
}
