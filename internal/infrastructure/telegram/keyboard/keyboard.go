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
	TeacherSetSelect   = "t_set"

	CabinetCurrentWeek = "cab_currweek"
	CabinetPrevDay     = "cab_prevday"
	CabinetNextDay     = "cab_nextday"

	ArchiveSelect    = "arch_sel"
	ArchiveBack      = "arch_back"
	NotifyToggle     = "notify_toggle"
	SubgroupSelect   = "subgroup_set"
	DaySubgroup      = "day_sg"
	WeekSubgroup     = "week_sg"
	WeekImage        = "week_img"
	TeacherWeekImage = "t_week_img"

	RoleSelect  = "role_sel"
	RoleStudent = "student"
	RoleTeacher = "teacher"

	BroadcastConfirm = "bcast_confirm"
	BroadcastCancel  = "bcast_cancel"

	btnCurrentWeek = "📆 Неделя"
	btnWeekImage   = "🖼 Картинкой"
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

func BuildReplyScheduleKeyboard(_ string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{
		ResizeKeyboard: true,
	}
	markup.ReplyKeyboard = scheduleReplyButtons()
	return markup
}

func InitKeyboard(webAppURL string) {
	ReplyScheduleKeyboard = BuildReplyScheduleKeyboard(webAppURL)
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

func InlineScheduleKeyboard(groupID int, week ...time.Time) *tele.ReplyMarkup {
	if groupID == 0 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	var weekBtn tele.Btn
	if len(week) > 0 && !week[0].IsZero() {
		weekBtn = markup.Data(btnCurrentWeek, NextWeek, fmt.Sprintf("%d:%s", groupID, week[0].Format("02.01.2006")))
	} else {
		weekBtn = markup.Data(btnCurrentWeek, CurrentWeek, strconv.Itoa(groupID))
	}
	markup.Inline(markup.Row(weekBtn))
	return markup
}

func InlineDayKeyboard(groupID, dayIdx, dayCount, subgroup int, weeksOpt ...entities.WeekNavigation) *tele.ReplyMarkup {
	if groupID == 0 {
		return nil
	}

	var weeks entities.WeekNavigation
	if len(weeksOpt) > 0 {
		weeks = weeksOpt[0]
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	var navRow []tele.Btn

	// Кнопка НАЗАД (btnPrev)
	if dayIdx > 0 {
		data := fmt.Sprintf("%d:%d:%d", groupID, dayIdx-1, subgroup)
		if !weeks.Current.IsZero() {
			data = fmt.Sprintf("%d:%s:%d:%d", groupID, weeks.Current.Format("02.01.2006"), dayIdx-1, subgroup)
		}
		navRow = append(navRow, markup.Data(btnPrev, PrevDay, data))
	} else if !weeks.Prev.IsZero() {
		data := fmt.Sprintf("%d:%s:-1:%d", groupID, weeks.Prev.Format("02.01.2006"), subgroup)
		navRow = append(navRow, markup.Data(btnPrev, PrevDay, data))
	}

	// Кнопка ПО ЦЕНТРУ (btnCurrentWeek / Вся неделя)
	var weekBtn tele.Btn
	if !weeks.Current.IsZero() {
		weekBtn = markup.Data(btnCurrentWeek, NextWeek, fmt.Sprintf("%d:%s:%d", groupID, weeks.Current.Format("02.01.2006"), subgroup))
	} else {
		weekBtn = markup.Data(btnCurrentWeek, CurrentWeek, fmt.Sprintf("%d:%d", groupID, subgroup))
	}
	navRow = append(navRow, weekBtn)

	// Кнопка ВПЕРЕД (btnNext)
	if dayIdx < dayCount-1 {
		data := fmt.Sprintf("%d:%d:%d", groupID, dayIdx+1, subgroup)
		if !weeks.Current.IsZero() {
			data = fmt.Sprintf("%d:%s:%d:%d", groupID, weeks.Current.Format("02.01.2006"), dayIdx+1, subgroup)
		}
		navRow = append(navRow, markup.Data(btnNext, NextDay, data))
	} else if !weeks.Next.IsZero() {
		data := fmt.Sprintf("%d:%s:0:%d", groupID, weeks.Next.Format("02.01.2006"), subgroup)
		navRow = append(navRow, markup.Data(btnNext, NextDay, data))
	}

	if len(navRow) > 1 || (len(navRow) == 1 && dayCount > 1) {
		rows = append(rows, navRow)
	}

	// 2-й ряд: 1 подгруппа | Вся группа | 2 подгруппа
	sg1Title := "1 подгруппа"
	sg0Title := "Вся группа"
	sg2Title := "2 подгруппа"
	switch subgroup {
	case 1:
		sg1Title = "• 1 подгруппа •"
	case 2:
		sg2Title = "• 2 подгруппа •"
	default:
		sg0Title = "• Вся группа •"
	}

	datePrefix := ""
	if !weeks.Current.IsZero() {
		datePrefix = weeks.Current.Format("02.01.2006") + ":"
	}

	rows = append(rows, markup.Row(
		markup.Data(sg1Title, DaySubgroup, fmt.Sprintf("%d:%s%d:1", groupID, datePrefix, dayIdx)),
		markup.Data(sg0Title, DaySubgroup, fmt.Sprintf("%d:%s%d:0", groupID, datePrefix, dayIdx)),
		markup.Data(sg2Title, DaySubgroup, fmt.Sprintf("%d:%s%d:2", groupID, datePrefix, dayIdx)),
	))

	if len(rows) == 0 {
		return nil
	}

	markup.Inline(rows...)
	return markup
}

func InlineWeekKeyboard(groupID int, weeks entities.WeekNavigation, subgroup int) *tele.ReplyMarkup {
	if groupID == 0 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	var navRow []tele.Btn
	if !weeks.Prev.IsZero() {
		navRow = append(navRow, markup.Data(btnPrev, PrevWeek, fmt.Sprintf("%d:%s:%d", groupID, weeks.Prev.Format("02.01.2006"), subgroup)))
	}
	if !weeks.Prev.IsZero() || !weeks.Next.IsZero() {
		navRow = append(navRow, markup.Data("Текущая", CurrentWeek, fmt.Sprintf("%d:%d", groupID, subgroup)))
	}
	if !weeks.Next.IsZero() {
		navRow = append(navRow, markup.Data(btnNext, NextWeek, fmt.Sprintf("%d:%s:%d", groupID, weeks.Next.Format("02.01.2006"), subgroup)))
	}

	if len(navRow) > 0 {
		rows = append(rows, markup.Row(navRow...))
	}

	// 2-й ряд: 1 подгруппа | Вся группа | 2 подгруппа
	sg1Title := "1 подгруппа"
	sg0Title := "Вся группа"
	sg2Title := "2 подгруппа"
	switch subgroup {
	case 1:
		sg1Title = "• 1 подгруппа •"
	case 2:
		sg2Title = "• 2 подгруппа •"
	default:
		sg0Title = "• Вся группа •"
	}

	dateStr := "current"
	if !weeks.Current.IsZero() {
		dateStr = weeks.Current.Format("02.01.2006")
	}

	rows = append(rows, markup.Row(
		markup.Data(sg1Title, WeekSubgroup, fmt.Sprintf("%d:%s:1", groupID, dateStr)),
		markup.Data(sg0Title, WeekSubgroup, fmt.Sprintf("%d:%s:0", groupID, dateStr)),
		markup.Data(sg2Title, WeekSubgroup, fmt.Sprintf("%d:%s:2", groupID, dateStr)),
	))

	// 3-й ряд: 🖼 Картинкой
	rows = append(rows, markup.Row(
		markup.Data(btnWeekImage, WeekImage, fmt.Sprintf("%d:%s:%d", groupID, dateStr, subgroup)),
	))

	markup.Inline(rows...)
	return markup
}

func InlineTeacherDayKeyboard(teacherName string, dayIdx, dayCount int, weeksOpt ...entities.WeekNavigation) *tele.ReplyMarkup {
	if teacherName == "" || dayCount <= 0 {
		return nil
	}

	var weeks entities.WeekNavigation
	if len(weeksOpt) > 0 {
		weeks = weeksOpt[0]
	}

	markup := &tele.ReplyMarkup{}
	var navRow []tele.Btn

	// Кнопка НАЗАД (btnPrev)
	if dayIdx > 0 {
		data := fmt.Sprintf("%s:%d", teacherName, dayIdx-1)
		if !weeks.Current.IsZero() {
			data = fmt.Sprintf("%s:%s:%d", teacherName, weeks.Current.Format("02.01.2006"), dayIdx-1)
		}
		navRow = append(navRow, markup.Data(btnPrev, TeacherPrevDay, data))
	} else if !weeks.Prev.IsZero() {
		data := fmt.Sprintf("%s:%s:-1", teacherName, weeks.Prev.Format("02.01.2006"))
		navRow = append(navRow, markup.Data(btnPrev, TeacherPrevDay, data))
	}

	// Кнопка ПО ЦЕНТРУ (btnCurrentWeek / Вся неделя)
	var weekBtn tele.Btn
	if !weeks.Current.IsZero() {
		weekBtn = markup.Data(btnCurrentWeek, TeacherNextWeek, fmt.Sprintf("%s:%s", teacherName, weeks.Current.Format("02.01.2006")))
	} else {
		weekBtn = markup.Data(btnCurrentWeek, TeacherCurrentWeek, teacherName)
	}
	navRow = append(navRow, weekBtn)

	// Кнопка ВПЕРЕД (btnNext)
	if dayIdx < dayCount-1 {
		data := fmt.Sprintf("%s:%d", teacherName, dayIdx+1)
		if !weeks.Current.IsZero() {
			data = fmt.Sprintf("%s:%s:%d", teacherName, weeks.Current.Format("02.01.2006"), dayIdx+1)
		}
		navRow = append(navRow, markup.Data(btnNext, TeacherNextDay, data))
	} else if !weeks.Next.IsZero() {
		data := fmt.Sprintf("%s:%s:0", teacherName, weeks.Next.Format("02.01.2006"))
		navRow = append(navRow, markup.Data(btnNext, TeacherNextDay, data))
	}

	if len(navRow) <= 1 && dayCount <= 1 {
		return nil
	}

	markup.Inline(markup.Row(navRow...))
	return markup
}

func InlineTeacherWeekKeyboard(teacherName string, weeks entities.WeekNavigation) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	var navRow []tele.Btn
	hasNav := !weeks.Prev.IsZero() || !weeks.Next.IsZero()

	if !weeks.Prev.IsZero() {
		navRow = append(navRow, markup.Data(btnPrev, TeacherPrevWeek, fmt.Sprintf("%s:%s", teacherName, weeks.Prev.Format("02.01.2006"))))
	}
	if hasNav {
		navRow = append(navRow, markup.Data("Текущая", TeacherCurrentWeek, teacherName))
	}
	if !weeks.Next.IsZero() {
		navRow = append(navRow, markup.Data(btnNext, TeacherNextWeek, fmt.Sprintf("%s:%s", teacherName, weeks.Next.Format("02.01.2006"))))
	}

	if len(navRow) > 0 {
		rows = append(rows, markup.Row(navRow...))
	}

	dateStr := "current"
	if !weeks.Current.IsZero() {
		dateStr = weeks.Current.Format("02.01.2006")
	}

	rows = append(rows, markup.Row(
		markup.Data(btnWeekImage, TeacherWeekImage, fmt.Sprintf("%s:%s", teacherName, dateStr)),
	))

	if len(rows) > 0 {
		markup.Inline(rows...)
	}
	return markup
}

func InlineTeacherScheduleKeyboard(teacherName string, week ...time.Time) *tele.ReplyMarkup {
	if teacherName == "" {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	var weekBtn tele.Btn
	if len(week) > 0 && !week[0].IsZero() {
		weekBtn = markup.Data(btnCurrentWeek, TeacherNextWeek, fmt.Sprintf("%s:%s", teacherName, week[0].Format("02.01.2006")))
	} else {
		weekBtn = markup.Data(btnCurrentWeek, TeacherCurrentWeek, teacherName)
	}
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

func InlineTeacherSetSuggestionsKeyboard(teachers []string) *tele.ReplyMarkup {
	if len(teachers) == 0 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, t := range teachers {
		btn := markup.Data(t, TeacherSetSelect, t)
		rows = append(rows, markup.Row(btn))
	}
	markup.Inline(rows...)
	return markup
}

func InlineCabinetDayKeyboard(cabinet string, dayIdx, dayCount int) *tele.ReplyMarkup {
	if cabinet == "" || dayCount <= 1 {
		return nil
	}

	markup := &tele.ReplyMarkup{}
	var navRow []tele.Btn

	if dayIdx > 0 {
		navRow = append(navRow, markup.Data(btnPrev, CabinetPrevDay, fmt.Sprintf("%s:%d", cabinet, dayIdx-1)))
	}

	navRow = append(navRow, markup.Data(btnCurrentWeek, CabinetCurrentWeek, cabinet))

	if dayIdx < dayCount-1 {
		navRow = append(navRow, markup.Data(btnNext, CabinetNextDay, fmt.Sprintf("%s:%d", cabinet, dayIdx+1)))
	}

	if len(navRow) <= 1 {
		return nil
	}

	markup.Inline(markup.Row(navRow...))
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

// InlineArchiveReturnKeyboard создает инлайн-кнопку возврата к списку недель архива.
func InlineArchiveReturnKeyboard(targetType, targetID string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	btn := markup.Data("📦 К выбору недели", ArchiveBack, targetType+":"+targetID)
	markup.Inline(markup.Row(btn))
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

func InlineRoleKeyboard() *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("🎓 Студент", RoleSelect, RoleStudent),
			markup.Data("👨‍🏫 Преподаватель", RoleSelect, RoleTeacher),
		),
	)
	return markup
}

func InlineBroadcastKeyboard() *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("✅ Отправить", BroadcastConfirm),
			markup.Data("❌ Отмена", BroadcastCancel),
		),
	)
	return markup
}
