package keyboard

import (
	"strings"
	"testing"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	tele "gopkg.in/telebot.v4"
)

// btnFull возвращает данные в том виде, в котором их получит бот: "<unique>|<args>".
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
	if got := InlineDayKeyboard(0, 2); got != nil {
		t.Errorf("InlineDayKeyboard(0, ...) = %v, want nil", got)
	}

	t.Run("offset 0 hides today button", func(t *testing.T) {
		m := InlineDayKeyboard(88, 0)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:0", "nextday|88:0", "currentweek|88"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})

	t.Run("offset non-zero shows today button", func(t *testing.T) {
		m := InlineDayKeyboard(88, -3)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:-3", "currentday|88", "nextday|88:-3", "currentweek|88"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})
}

func TestInlineWeekKeyboard(t *testing.T) {
	current := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)

	t.Run("no weeks available", func(t *testing.T) {
		m := InlineWeekKeyboard(99, entities.Weeks{})
		if m == nil || len(m.InlineKeyboard) != 0 {
			t.Errorf("expected empty inline keyboard, got %+v", m)
		}
	})

	t.Run("both directions", func(t *testing.T) {
		weeks := entities.Weeks{
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

	t.Run("only next week", func(t *testing.T) {
		weeks := entities.Weeks{Current: current, Next: current.AddDate(0, 0, 7)}

		m := InlineWeekKeyboard(88, weeks)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"currentweek|88", "nextweek|88:31.08.2026"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})
}
