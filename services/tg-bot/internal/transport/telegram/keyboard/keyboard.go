package keyboard

import (
	"fmt"
	"strconv"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	tele "gopkg.in/telebot.v4"
)

const (
	BtnDay      = "📅 День"
	BtnWeek     = "📆 Неделя"
	BtnCalls    = "⏰ Звонки"
	CurrentWeek = "currentweek"
	PrevWeek    = "prevweek"
	NextWeek    = "nextweek"
	PrevDay     = "prevday"
	NextDay     = "nextday"

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
func InlineDayKeyboard(groupID, dayIdx int) *tele.ReplyMarkup {
	if groupID == 0 {
		return nil
	}

	markup := &tele.ReplyMarkup{}

	prev := (dayIdx + 5) % 6
	next := (dayIdx + 1) % 6

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

func groupIDString(groupID int) string {
	return strconv.Itoa(groupID)
}

func scheduleReplyButtons() [][]tele.ReplyButton {
	return [][]tele.ReplyButton{
		{tele.ReplyButton{Text: BtnDay}, tele.ReplyButton{Text: BtnWeek}},
		{tele.ReplyButton{Text: BtnCalls}},
	}
}
