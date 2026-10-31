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

	t.Run("with target week uses NextWeek callback and formatted date", func(t *testing.T) {
		targetWeek := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
		mw := InlineScheduleKeyboard(99, targetWeek)
		if mw == nil || len(mw.InlineKeyboard) != 1 || len(mw.InlineKeyboard[0]) != 1 {
			t.Fatalf("unexpected keyboard structure: %+v", mw)
		}
		btnW := mw.InlineKeyboard[0][0]
		if btnW.Unique != NextWeek || btnW.Data != "99:02.11.2026" {
			t.Errorf("callback = %s|%s, want %s|99:02.11.2026", btnW.Unique, btnW.Data, NextWeek)
		}
	})
}

func TestInlineTeacherScheduleKeyboard(t *testing.T) {
	if got := InlineTeacherScheduleKeyboard(""); got != nil {
		t.Errorf("InlineTeacherScheduleKeyboard(\"\") = %v, want nil", got)
	}

	m := InlineTeacherScheduleKeyboard("Иванов И. И.")
	if m == nil || len(m.InlineKeyboard) != 1 || len(m.InlineKeyboard[0]) != 1 {
		t.Fatalf("unexpected keyboard structure: %+v", m)
	}
	btn := m.InlineKeyboard[0][0]
	if btn.Unique != TeacherCurrentWeek || btn.Data != "Иванов И. И." {
		t.Errorf("got %s|%s, want %s|Иванов И. И.", btn.Unique, btn.Data, TeacherCurrentWeek)
	}

	t.Run("with target week uses TeacherNextWeek callback and formatted date", func(t *testing.T) {
		targetWeek := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
		mw := InlineTeacherScheduleKeyboard("Иванов И. И.", targetWeek)
		if mw == nil || len(mw.InlineKeyboard) != 1 || len(mw.InlineKeyboard[0]) != 1 {
			t.Fatalf("unexpected keyboard structure: %+v", mw)
		}
		btnW := mw.InlineKeyboard[0][0]
		if btnW.Unique != TeacherNextWeek || btnW.Data != "Иванов И. И.:02.11.2026" {
			t.Errorf("got %s|%s, want %s|Иванов И. И.:02.11.2026", btnW.Unique, btnW.Data, TeacherNextWeek)
		}
	})
}

func TestInlineDayKeyboard(t *testing.T) {
	if got := InlineDayKeyboard(0, 2, 6, 0); got != nil {
		t.Errorf("InlineDayKeyboard(0, ...) = %v, want nil", got)
	}

	t.Run("monday without prev week hides prev button", func(t *testing.T) {
		m := InlineDayKeyboard(88, 0, 6, 0)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"currentweek|88:0", "nextday|88:1:0"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}

		sgDatas := btnFull(t, m.InlineKeyboard[1])
		wantSg := []string{"day_sg|88:0:1", "day_sg|88:0:0", "day_sg|88:0:2"}
		if strings.Join(sgDatas, ",") != strings.Join(wantSg, ",") {
			t.Errorf("subgroup buttons = %v, want %v", sgDatas, wantSg)
		}
	})

	t.Run("saturday without next week hides next button", func(t *testing.T) {
		m := InlineDayKeyboard(88, 5, 6, 1)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:4:1", "currentweek|88:1"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})

	t.Run("midweek has both prev and next", func(t *testing.T) {
		m := InlineDayKeyboard(88, 2, 6, 0)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:1:0", "currentweek|88:0", "nextday|88:3:0"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("buttons = %v, want %v", datas, want)
		}
	})

	t.Run("monday with prev week points prev to previous week saturday", func(t *testing.T) {
		prev := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
		curr := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
		next := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)

		weeks := entities.WeekNavigation{
			Prev:    prev,
			Current: curr,
			Next:    next,
		}

		m := InlineDayKeyboard(88, 0, 6, 0, weeks)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:12.10.2026:-1:0", "nextweek|88:19.10.2026:0", "nextday|88:19.10.2026:1:0"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}

		sgDatas := btnFull(t, m.InlineKeyboard[1])
		wantSg := []string{"day_sg|88:19.10.2026:0:1", "day_sg|88:19.10.2026:0:0", "day_sg|88:19.10.2026:0:2"}
		if strings.Join(sgDatas, ",") != strings.Join(wantSg, ",") {
			t.Errorf("subgroup buttons = %v, want %v", sgDatas, wantSg)
		}
	})

	t.Run("saturday with next week points next to next week monday", func(t *testing.T) {
		prev := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
		curr := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
		next := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)

		weeks := entities.WeekNavigation{
			Prev:    prev,
			Current: curr,
			Next:    next,
		}

		m := InlineDayKeyboard(88, 5, 6, 2, weeks)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevday|88:19.10.2026:4:2", "nextweek|88:19.10.2026:2", "nextday|88:26.10.2026:0:2"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}
	})
}

func TestInlineTeacherDayKeyboard_CrossWeek(t *testing.T) {
	prev := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	next := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)

	t.Run("monday with prev week", func(t *testing.T) {
		weeks := entities.WeekNavigation{Prev: prev, Current: curr}
		m := InlineTeacherDayKeyboard("Иванов И. И.", 0, 6, weeks)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"t_prevday|Иванов И. И.:12.10.2026:-1", "t_nextweek|Иванов И. И.:19.10.2026", "t_nextday|Иванов И. И.:19.10.2026:1"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}
	})

	t.Run("saturday with next week", func(t *testing.T) {
		weeks := entities.WeekNavigation{Current: curr, Next: next}
		m := InlineTeacherDayKeyboard("Иванов И. И.", 5, 6, weeks)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"t_prevday|Иванов И. И.:19.10.2026:4", "t_nextweek|Иванов И. И.:19.10.2026", "t_nextday|Иванов И. И.:26.10.2026:0"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}
	})

	t.Run("monday without prev week hides prev", func(t *testing.T) {
		m := InlineTeacherDayKeyboard("Иванов И. И.", 0, 6)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"t_currweek|Иванов И. И.", "t_nextday|Иванов И. И.:1"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}
	})

	t.Run("saturday without next week hides next", func(t *testing.T) {
		m := InlineTeacherDayKeyboard("Иванов И. И.", 5, 6)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"t_prevday|Иванов И. И.:4", "t_currweek|Иванов И. И."}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}
	})
}

func TestInlineWeekKeyboard(t *testing.T) {
	current := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)

	t.Run("no weeks available has subgroup row and image button", func(t *testing.T) {
		m := InlineWeekKeyboard(99, entities.WeekNavigation{}, 0)
		if m == nil || len(m.InlineKeyboard) != 2 {
			t.Fatalf("expected 2 rows (subgroups and image button), got %+v", m)
		}
		sgDatas := btnFull(t, m.InlineKeyboard[0])
		wantSg := []string{"week_sg|99:current:1", "week_sg|99:current:0", "week_sg|99:current:2"}
		if strings.Join(sgDatas, ",") != strings.Join(wantSg, ",") {
			t.Errorf("subgroup buttons = %v, want %v", sgDatas, wantSg)
		}

		imgDatas := btnFull(t, m.InlineKeyboard[1])
		wantImg := []string{"week_img|99:current:0"}
		if strings.Join(imgDatas, ",") != strings.Join(wantImg, ",") {
			t.Errorf("img button = %v, want %v", imgDatas, wantImg)
		}
	})

	t.Run("both directions", func(t *testing.T) {
		weeks := entities.WeekNavigation{
			Prev:    current.AddDate(0, 0, -7),
			Current: current,
			Next:    current.AddDate(0, 0, 7),
		}

		m := InlineWeekKeyboard(88, weeks, 0)
		datas := btnFull(t, m.InlineKeyboard[0])

		want := []string{"prevweek|88:17.08.2026:0", "currentweek|88:0", "nextweek|88:31.08.2026:0"}
		if strings.Join(datas, ",") != strings.Join(want, ",") {
			t.Errorf("nav buttons = %v, want %v", datas, want)
		}

		sgDatas := btnFull(t, m.InlineKeyboard[1])
		wantSg := []string{"week_sg|88:24.08.2026:1", "week_sg|88:24.08.2026:0", "week_sg|88:24.08.2026:2"}
		if strings.Join(sgDatas, ",") != strings.Join(wantSg, ",") {
			t.Errorf("subgroup buttons = %v, want %v", sgDatas, wantSg)
		}

		imgDatas := btnFull(t, m.InlineKeyboard[2])
		wantImg := []string{"week_img|88:24.08.2026:0"}
		if strings.Join(imgDatas, ",") != strings.Join(wantImg, ",") {
			t.Errorf("img button = %v, want %v", imgDatas, wantImg)
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
	if len(markupWithApp.ReplyKeyboard) != 2 {
		t.Errorf("expected 2 rows even with webApp URL (app button removed), got %d", len(markupWithApp.ReplyKeyboard))
	}
}

func TestInlineArchiveWeeksKeyboard(t *testing.T) {
	if got := InlineArchiveWeeksKeyboard("g", "101", nil); got != nil {
		t.Errorf("expected nil for empty weeks, got %v", got)
	}

	w1, _ := time.Parse("2006-01-02", "2026-10-12")
	w2, _ := time.Parse("2006-01-02", "2026-10-05")
	m := InlineArchiveWeeksKeyboard("g", "101", []time.Time{w1, w2})
	if m == nil || len(m.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %v", m)
	}

	btn := m.InlineKeyboard[0][0]
	if btn.Unique+"|"+btn.Data != "arch_sel|g:101:12.10.2026" {
		t.Errorf("unexpected callback data: %s|%s", btn.Unique, btn.Data)
	}
	if !strings.Contains(btn.Text, "12.10 — 17.10.2026") {
		t.Errorf("unexpected button label: %s", btn.Text)
	}
}

func TestInlineNotifyKeyboard(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		m := InlineNotifyKeyboard(true)
		if m == nil || len(m.InlineKeyboard) != 1 || len(m.InlineKeyboard[0]) != 1 {
			t.Fatalf("unexpected keyboard structure: %+v", m)
		}
		btn := m.InlineKeyboard[0][0]
		if btn.Unique+"|"+btn.Data != "notify_toggle|off" {
			t.Errorf("btn callback = %s|%s, want notify_toggle|off", btn.Unique, btn.Data)
		}
		if !strings.Contains(btn.Text, "Отключить") {
			t.Errorf("btn.Text = %q, want Отключить", btn.Text)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		m := InlineNotifyKeyboard(false)
		if m == nil || len(m.InlineKeyboard) != 1 || len(m.InlineKeyboard[0]) != 1 {
			t.Fatalf("unexpected keyboard structure: %+v", m)
		}
		btn := m.InlineKeyboard[0][0]
		if btn.Unique+"|"+btn.Data != "notify_toggle|on" {
			t.Errorf("btn callback = %s|%s, want notify_toggle|on", btn.Unique, btn.Data)
		}
		if !strings.Contains(btn.Text, "Включить") {
			t.Errorf("btn.Text = %q, want Включить", btn.Text)
		}
	})
}

func TestInlineSubgroupKeyboard(t *testing.T) {
	m := InlineSubgroupKeyboard(101)
	if m == nil {
		t.Fatal("expected non-nil keyboard")
	}

	if len(m.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(m.InlineKeyboard))
	}

	row1 := m.InlineKeyboard[0]
	if len(row1) != 2 {
		t.Fatalf("expected 2 buttons in row 1, got %d", len(row1))
	}
	if row1[0].Data != "101:1" || !strings.Contains(row1[0].Text, "1 подгруппа") {
		t.Errorf("button 1 mismatch: data=%s text=%s", row1[0].Data, row1[0].Text)
	}
	if row1[1].Data != "101:2" || !strings.Contains(row1[1].Text, "2 подгруппа") {
		t.Errorf("button 2 mismatch: data=%s text=%s", row1[1].Data, row1[1].Text)
	}

	row2 := m.InlineKeyboard[1]
	if len(row2) != 1 {
		t.Fatalf("expected 1 button in row 2, got %d", len(row2))
	}
	if row2[0].Data != "101:0" || !strings.Contains(row2[0].Text, "Вся группа") {
		t.Errorf("button all mismatch: data=%s text=%s", row2[0].Data, row2[0].Text)
	}
}

func TestInlineTeacherSuggestionsKeyboard(t *testing.T) {
	if got := InlineTeacherSuggestionsKeyboard(nil); got != nil {
		t.Errorf("InlineTeacherSuggestionsKeyboard(nil) = %v, want nil", got)
	}
	if got := InlineTeacherSuggestionsKeyboard([]string{}); got != nil {
		t.Errorf("InlineTeacherSuggestionsKeyboard(empty) = %v, want nil", got)
	}

	teachers := []string{"Анципов Е. Ю.", "Иванов И. И.", "Калитеня И. И."}
	m := InlineTeacherSuggestionsKeyboard(teachers)
	if m == nil {
		t.Fatal("expected non-nil keyboard")
	}

	if len(m.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(m.InlineKeyboard))
	}

	for i, name := range teachers {
		row := m.InlineKeyboard[i]
		if len(row) != 1 {
			t.Fatalf("row %d: expected 1 button, got %d", i, len(row))
		}
		btn := row[0]
		if btn.Text != name {
			t.Errorf("row %d: btn.Text = %q, want %q", i, btn.Text, name)
		}
		if btn.Unique != TeacherCurrentWeek {
			t.Errorf("row %d: btn.Unique = %q, want %q", i, btn.Unique, TeacherCurrentWeek)
		}
		if btn.Data != name {
			t.Errorf("row %d: btn.Data = %q, want %q", i, btn.Data, name)
		}
	}
}

func TestInlineTeacherSetSuggestionsKeyboard(t *testing.T) {
	if got := InlineTeacherSetSuggestionsKeyboard(nil); got != nil {
		t.Errorf("InlineTeacherSetSuggestionsKeyboard(nil) = %v, want nil", got)
	}
	if got := InlineTeacherSetSuggestionsKeyboard([]string{}); got != nil {
		t.Errorf("InlineTeacherSetSuggestionsKeyboard(empty) = %v, want nil", got)
	}

	teachers := []string{"Анципов Е. Ю.", "Иванов И. И."}
	m := InlineTeacherSetSuggestionsKeyboard(teachers)
	if m == nil {
		t.Fatal("expected non-nil keyboard")
	}

	if len(m.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(m.InlineKeyboard))
	}

	for i, name := range teachers {
		row := m.InlineKeyboard[i]
		if len(row) != 1 {
			t.Fatalf("row %d: expected 1 button, got %d", i, len(row))
		}
		btn := row[0]
		if btn.Text != name {
			t.Errorf("row %d: btn.Text = %q, want %q", i, btn.Text, name)
		}
		if btn.Unique != TeacherSetSelect {
			t.Errorf("row %d: btn.Unique = %q, want %q", i, btn.Unique, TeacherSetSelect)
		}
		if btn.Data != name {
			t.Errorf("row %d: btn.Data = %q, want %q", i, btn.Data, name)
		}
	}
}

func TestInlineRoleKeyboard(t *testing.T) {
	m := InlineRoleKeyboard()
	if m == nil {
		t.Fatal("expected non-nil keyboard")
	}

	if len(m.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(m.InlineKeyboard))
	}

	row := m.InlineKeyboard[0]
	if len(row) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(row))
	}

	btnStudent := row[0]
	if btnStudent.Text != "🎓 Студент" {
		t.Errorf("student btn.Text = %q, want %q", btnStudent.Text, "🎓 Студент")
	}
	if btnStudent.Unique != RoleSelect {
		t.Errorf("student btn.Unique = %q, want %q", btnStudent.Unique, RoleSelect)
	}
	if btnStudent.Data != RoleStudent {
		t.Errorf("student btn.Data = %q, want %q", btnStudent.Data, RoleStudent)
	}

	btnTeacher := row[1]
	if btnTeacher.Text != "👨‍🏫 Преподаватель" {
		t.Errorf("teacher btn.Text = %q, want %q", btnTeacher.Text, "👨‍🏫 Преподаватель")
	}
	if btnTeacher.Unique != RoleSelect {
		t.Errorf("teacher btn.Unique = %q, want %q", btnTeacher.Unique, RoleSelect)
	}
	if btnTeacher.Data != RoleTeacher {
		t.Errorf("teacher btn.Data = %q, want %q", btnTeacher.Data, RoleTeacher)
	}
}

func TestInlineTeacherWeekKeyboard(t *testing.T) {
	curr := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	prev := curr.AddDate(0, 0, -7)
	next := curr.AddDate(0, 0, 7)

	weeks := entities.WeekNavigation{
		Current: curr,
		Prev:    prev,
		Next:    next,
	}

	m := InlineTeacherWeekKeyboard("Камлюк В. С.", weeks)
	if m == nil {
		t.Fatal("expected non-nil keyboard")
	}

	// Ожидается как минимум строка навигации и строка с кнопкой "Картинкой"
	if len(m.InlineKeyboard) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(m.InlineKeyboard))
	}

	// Проверяем строку с кнопкой картинки
	lastRow := m.InlineKeyboard[len(m.InlineKeyboard)-1]
	if len(lastRow) != 1 {
		t.Fatalf("expected 1 button in last row, got %d", len(lastRow))
	}

	imgBtn := lastRow[0]
	if imgBtn.Unique != TeacherWeekImage {
		t.Errorf("imgBtn.Unique = %q, want %q", imgBtn.Unique, TeacherWeekImage)
	}
	if imgBtn.Text != btnWeekImage {
		t.Errorf("imgBtn.Text = %q, want %q", imgBtn.Text, btnWeekImage)
	}
	expectedData := "Камлюк В. С.:02.11.2026"
	if imgBtn.Data != expectedData {
		t.Errorf("imgBtn.Data = %q, want %q", imgBtn.Data, expectedData)
	}
}
