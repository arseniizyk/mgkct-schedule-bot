package keyboard

import (
	"fmt"
	"strconv"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

const (
	BtnDay      = "📅 День"
	BtnWeek     = "📆 Неделя"
	BtnCalls    = "⏰ Звонки"
	BtnTeachers = "👨‍🏫 Преподаватели"
	BtnApp      = "📱 Приложение"

	CurrentWeek = "currentweek"
	PrevWeek    = "prevweek"
	NextWeek    = "nextweek"
	PrevDay     = "prevday"
	NextDay     = "nextday"

	TeacherCurrentWeek = "t_currweek"
	TeacherPrevWeek    = "t_prevweek"
	TeacherNextWeek    = "t_nextweek"
	TeacherPrevDay     = "t_prevday"
	TeacherNextDay     = "t_nextday"

	btnCurrentWeek = "📆 Неделя"
	btnPrev        = "◀️"
	btnNext        = "▶️"
)

var (
	InlineEmptyKeyboard   = &tele.ReplyMarkup{}
	ReplyScheduleKeyboard = &tele.ReplyMarkup{
		ResizeKeyboard: true,
		ReplyKeyboard:  scheduleReplyButtons(),
		IsPersistent:   true,
	}
)

func BuildReplyScheduleKeyboard(webAppURL string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
	var rows [][]tele.ReplyButton
	if webAppURL != "" {
		rows = append(rows, []tele.ReplyButton{
			{Text: BtnApp, WebApp: &tele.WebApp{URL: webAppURL}},
		})
	}
	rows = append(rows,
		[]tele.ReplyButton{{Text: BtnDay}, {Text: BtnWeek}},
		[]tele.ReplyButton{{Text: BtnCalls}, {Text: BtnTeachers}},
	)
	markup.ReplyKeyboard = rows
	return markup
}

func InitKeyboard(webAppURL string) {
	if webAppURL != "" {
		ReplyScheduleKeyboard = BuildReplyScheduleKeyboard(webAppURL)
	}
}

func InlineWebAppKeyboard(webAppURL string) *tele.ReplyMarkup {
	if webAppURL == "" {
		return nil
	}
	markup := &tele.ReplyMarkup{}
	btn := markup.WebApp("📱 Открыть в приложении", &tele.WebApp{URL: webAppURL})
	markup.Inline(markup.Row(btn))
	return markup
}

func InlineScheduleKeyboard(groupID int) *tele.ReplyMarkup {
	if groupID == 0 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	weekBtn := markup.Data(btnCurrentWeek, CurrentWeek, strconv.Itoa(groupID))
	markup.Inline(markup.Row(weekBtn))
	return markup
}

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

func InlineWeekKeyboard(groupID int, weeks entities.WeekNavigation) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var row []tele.Btn

	hasNav := !weeks.Prev.IsZero() || !weeks.Next.IsZero()

	if !weeks.Prev.IsZero() {
		row = append(row, markup.Data(btnPrev, PrevWeek, fmt.Sprintf("%d:%s", groupID, weeks.Prev.Format("02.01.2006"))))
	}
	if hasNav {
		row = append(row, markup.Data("Текущая", CurrentWeek, strconv.Itoa(groupID)))
	}
	if !weeks.Next.IsZero() {
		row = append(row, markup.Data(btnNext, NextWeek, fmt.Sprintf("%d:%s", groupID, weeks.Next.Format("02.01.2006"))))
	}

	if len(row) > 0 {
		markup.Inline(markup.Row(row...))
	}
	return markup
}

func InlineTeacherDayKeyboard(teacherName string, dayIdx, dayCount int) *tele.ReplyMarkup {
	if dayCount <= 1 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	prev := (dayIdx + dayCount - 1) % dayCount
	next := (dayIdx + 1) % dayCount

	markup.Inline(markup.Row(
		markup.Data(btnPrev, TeacherPrevDay, fmt.Sprintf("%s:%d", teacherName, prev)),
		markup.Data(btnCurrentWeek, TeacherCurrentWeek, teacherName),
		markup.Data(btnNext, TeacherNextDay, fmt.Sprintf("%s:%d", teacherName, next)),
	))

	return markup
}

func InlineTeacherWeekKeyboard(teacherName string, weeks entities.WeekNavigation) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var row []tele.Btn

	hasNav := !weeks.Prev.IsZero() || !weeks.Next.IsZero()

	if !weeks.Prev.IsZero() {
		row = append(row, markup.Data(btnPrev, TeacherPrevWeek, fmt.Sprintf("%s:%s", teacherName, weeks.Prev.Format("02.01.2006"))))
	}
	if hasNav {
		row = append(row, markup.Data("Текущая", TeacherCurrentWeek, teacherName))
	}
	if !weeks.Next.IsZero() {
		row = append(row, markup.Data(btnNext, TeacherNextWeek, fmt.Sprintf("%s:%s", teacherName, weeks.Next.Format("02.01.2006"))))
	}

	if len(row) > 0 {
		markup.Inline(markup.Row(row...))
	}
	return markup
}

func InlineTeacherScheduleKeyboard(teacherName string) *tele.ReplyMarkup {
	if teacherName == "" {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	weekBtn := markup.Data(btnCurrentWeek, TeacherCurrentWeek, teacherName)
	markup.Inline(markup.Row(weekBtn))
	return markup
}

func scheduleReplyButtons() [][]tele.ReplyButton {
	return [][]tele.ReplyButton{
		{tele.ReplyButton{Text: BtnDay}, tele.ReplyButton{Text: BtnWeek}},
		{tele.ReplyButton{Text: BtnCalls}, tele.ReplyButton{Text: BtnTeachers}},
	}
}
