package keyboard

import (
	"fmt"
	"strconv"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

const (
	BtnDay      = "📅 День"
	BtnWeek     = "📆 Неделя"
	BtnCalls    = "⏰ Звонки"
	BtnTeachers = "👨‍🏫 Преподаватели"
	BtnApp      = "Приложение"

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

	CabinetCurrentWeek = "cab_currweek"
	CabinetPrevDay     = "cab_prevday"
	CabinetNextDay     = "cab_nextday"

	ArchiveSelect  = "arch_sel"
	NotifyToggle   = "notify_toggle"
	SubgroupSelect = "subgroup_set"

	btnCurrentWeek = "📆 Неделя"
	btnPrev        = "◀️"
	btnNext        = "▶️"
)

var (
	InlineEmptyKeyboard   = &tele.ReplyMarkup{}
	ReplyScheduleKeyboard = &tele.ReplyMarkup{
		ResizeKeyboard: true,
		ReplyKeyboard:  scheduleReplyButtons(),
	}
)

func BuildReplyScheduleKeyboard(webAppURL string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{
		ResizeKeyboard: true,
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
	btn := markup.WebApp("Открыть в приложении", &tele.WebApp{URL: webAppURL})
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

func InlineTeacherSuggestionsKeyboard(teachers []string) *tele.ReplyMarkup {
	if len(teachers) == 0 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, t := range teachers {
		btn := markup.Data(t, TeacherCurrentWeek, t)
		rows = append(rows, markup.Row(btn))
	}
	markup.Inline(rows...)
	return markup
}

func InlineCabinetDayKeyboard(cabinet string, dayIdx, dayCount int) *tele.ReplyMarkup {
	if dayCount <= 1 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	prev := (dayIdx + dayCount - 1) % dayCount
	next := (dayIdx + 1) % dayCount

	markup.Inline(markup.Row(
		markup.Data(btnPrev, CabinetPrevDay, fmt.Sprintf("%s:%d", cabinet, prev)),
		markup.Data(btnCurrentWeek, CabinetCurrentWeek, cabinet),
		markup.Data(btnNext, CabinetNextDay, fmt.Sprintf("%s:%d", cabinet, next)),
	))

	return markup
}

func InlineCabinetWeekKeyboard(cabinet string) *tele.ReplyMarkup {
	if cabinet == "" {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	weekBtn := markup.Data(btnCurrentWeek, CabinetCurrentWeek, cabinet)
	markup.Inline(markup.Row(weekBtn))
	return markup
}

func scheduleReplyButtons() [][]tele.ReplyButton {
	return [][]tele.ReplyButton{
		{tele.ReplyButton{Text: BtnDay}, tele.ReplyButton{Text: BtnWeek}},
		{tele.ReplyButton{Text: BtnCalls}, tele.ReplyButton{Text: BtnTeachers}},
	}
}

// InlineArchiveWeeksKeyboard создает инлайн-клавиатуру для выбора недели из архива
// targetType: "g" (group) или "t" (teacher)
// targetID: "101" или имя преподавателя
func InlineArchiveWeeksKeyboard(targetType, targetID string, weeks []time.Time) *tele.ReplyMarkup {
	if len(weeks) == 0 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	maxWeeks := min(len(weeks), 10)

	for i := range maxWeeks {
		w := weeks[i]
		endW := w.AddDate(0, 0, 5)
		title := fmt.Sprintf("📅 %s — %s", w.Format("02.01"), endW.Format("02.01.2006"))
		data := fmt.Sprintf("%s:%s:%s", targetType, targetID, w.Format("02.01.2006"))
		btn := markup.Data(title, ArchiveSelect, data)
		rows = append(rows, markup.Row(btn))
	}

	markup.Inline(rows...)
	return markup
}

func InlineNotifyKeyboard(enabled bool) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var btn tele.Btn
	if enabled {
		btn = markup.Data("🔕 Отключить вечернюю рассылку", NotifyToggle, "off")
	} else {
		btn = markup.Data("🔔 Включить вечернюю рассылку", NotifyToggle, "on")
	}
	markup.Inline(markup.Row(btn))
	return markup
}

func InlineSubgroupKeyboard(groupID int) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("1 подгруппа", SubgroupSelect, fmt.Sprintf("%d:1", groupID)),
			markup.Data("2 подгруппа", SubgroupSelect, fmt.Sprintf("%d:2", groupID)),
		),
		markup.Row(
			markup.Data("Вся группа", SubgroupSelect, fmt.Sprintf("%d:0", groupID)),
		),
	)
	return markup
}
