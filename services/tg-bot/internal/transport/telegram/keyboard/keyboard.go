package keyboard

import (
	"fmt"
	"strconv"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
)

const (
	BtnDay      = "📅 День"
	BtnWeek     = "📆 Неделя"
	BtnCalls    = "⏰ Звонки"
	BtnTeachers = "👨‍🏫 Преподаватели"
	CurrentWeek = "currentweek"
	PrevWeek    = "prevweek"
	NextWeek    = "nextweek"
	PrevDay     = "prevday"
	NextDay     = "nextday"

	TeacherCurrentWeek = "teacher_currentweek"
	TeacherPrevWeek    = "teacher_prevweek"
	TeacherNextWeek    = "teacher_nextweek"
	TeacherPrevDay     = "teacher_prevday"
	TeacherNextDay     = "teacher_nextday"

	btnCurrentWeek = "📆 Неделя"
	btnPrev        = "◀️"
	btnNext        = "▶️"
)

// Общие клавиатуры — создаются один раз на старте и переиспользуются между сообщениями.
//
//nolint:gochecknoglobals // намеренные разделяемые синглтоны
var (
	InlineEmptyKeyboard   = &tele.ReplyMarkup{}
	ReplyScheduleKeyboard = &tele.ReplyMarkup{
		ResizeKeyboard: true,
		ReplyKeyboard:  scheduleReplyButtons(),
		IsPersistent:   true,
	}
)

// InlineScheduleKeyboard — кнопка раскрытия недельного вида (уведомления о новой неделе).
func InlineScheduleKeyboard(groupID int) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	if groupID == 0 {
		return nil
	}

	weekBtn := markup.Data(btnCurrentWeek, CurrentWeek, strconv.Itoa(groupID))
	markup.Inline(markup.Row(weekBtn))

	return markup
}

// InlineDayKeyboard — навигация по дням одной строкой:
// ◀️ (слева), «Неделя» (в центре), ▶️ (справа). Стрелки несут индекс дня,
// в который ведут, поэтому каждый шаг гарантированно меняет день.
// dayCount — реальное число дней в расписании (не предполагается ровно 6).
func InlineDayKeyboard(groupID, dayIdx, dayCount int) *tele.ReplyMarkup {
	if groupID == 0 || dayCount <= 1 {
		return nil
	}

	markup := &tele.ReplyMarkup{}

	prev := (dayIdx + dayCount - 1) % dayCount
	next := (dayIdx + 1) % dayCount

	markup.Inline(markup.Row(
		markup.Data(btnPrev, PrevDay, fmt.Sprintf("%d:%d", groupID, prev)),
		markup.Data(btnCurrentWeek, CurrentWeek, strconv.Itoa(groupID)),
		markup.Data(btnNext, NextDay, fmt.Sprintf("%d:%d", groupID, next)),
	))

	return markup
}

// InlineWeekKeyboard — навигация по неделям: ◀️ / Текущая / ▶️.
func InlineWeekKeyboard(groupID int, weeks entities.Weeks) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var row []tele.Btn

	hasNav := !weeks.Prev.IsZero() || !weeks.Next.IsZero()

	if !weeks.Prev.IsZero() {
		row = append(row, markup.Data(btnPrev, PrevWeek, fmt.Sprintf("%s:%s", strconv.Itoa(groupID), weeks.Prev.Format("02.01.2006"))))
	}
	if hasNav {
		row = append(row, markup.Data("Текущая", CurrentWeek, groupIDString(groupID)))
	}
	if !weeks.Next.IsZero() {
		row = append(row, markup.Data(btnNext, NextWeek, fmt.Sprintf("%s:%s", strconv.Itoa(groupID), weeks.Next.Format("02.01.2006"))))
	}

	if len(row) > 0 {
		markup.Inline(markup.Row(row...))
	}
	return markup
}

// InlineTeacherDayKeyboard — навигация по дням для расписания преподавателя.
// В callback_data передаётся chatID (числовой), а не ФИО преподавателя, чтобы
// уложиться в 64-байтовый лимит Telegram и не зависеть от длины кириллического имени.
// dayCount — реальное число дней в расписании (не предполагается ровно 6).
func InlineTeacherDayKeyboard(chatID int64, dayIdx, dayCount int) *tele.ReplyMarkup {
	if dayCount <= 1 {
		return nil
	}

	markup := &tele.ReplyMarkup{}

	prev := (dayIdx + dayCount - 1) % dayCount
	next := (dayIdx + 1) % dayCount

	markup.Inline(markup.Row(
		markup.Data(btnPrev, TeacherPrevDay, fmt.Sprintf("%d:%d", chatID, prev)),
		markup.Data(btnCurrentWeek, TeacherCurrentWeek, strconv.FormatInt(chatID, 10)),
		markup.Data(btnNext, TeacherNextDay, fmt.Sprintf("%d:%d", chatID, next)),
	))

	return markup
}

// InlineTeacherWeekKeyboard — навигация по неделям для расписания преподавателя.
func InlineTeacherWeekKeyboard(chatID int64, weeks entities.Weeks) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var row []tele.Btn

	hasNav := !weeks.Prev.IsZero() || !weeks.Next.IsZero()

	if !weeks.Prev.IsZero() {
		row = append(row, markup.Data(btnPrev, TeacherPrevWeek, fmt.Sprintf("%d:%s", chatID, weeks.Prev.Format("02.01.2006"))))
	}
	if hasNav {
		row = append(row, markup.Data("Текущая", TeacherCurrentWeek, strconv.FormatInt(chatID, 10)))
	}
	if !weeks.Next.IsZero() {
		row = append(row, markup.Data(btnNext, TeacherNextWeek, fmt.Sprintf("%d:%s", chatID, weeks.Next.Format("02.01.2006"))))
	}

	if len(row) > 0 {
		markup.Inline(markup.Row(row...))
	}
	return markup
}

// InlineTeacherScheduleKeyboard — кнопка раскрытия недельного вида для уведомлений преподавателя.
func InlineTeacherScheduleKeyboard(chatID int64) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	if chatID == 0 {
		return nil
	}

	weekBtn := markup.Data(btnCurrentWeek, TeacherCurrentWeek, strconv.FormatInt(chatID, 10))
	markup.Inline(markup.Row(weekBtn))

	return markup
}

func groupIDString(groupID int) string {
	return strconv.Itoa(groupID)
}

func scheduleReplyButtons() [][]tele.ReplyButton {
	return [][]tele.ReplyButton{
		{tele.ReplyButton{Text: BtnDay}, tele.ReplyButton{Text: BtnWeek}},
		{tele.ReplyButton{Text: BtnCalls}, tele.ReplyButton{Text: BtnTeachers}},
	}
}
