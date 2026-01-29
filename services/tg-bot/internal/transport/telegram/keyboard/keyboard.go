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
)

var (
	InlineEmptyKeyboard   = &tele.ReplyMarkup{}
	ReplyScheduleKeyboard = &tele.ReplyMarkup{
		ResizeKeyboard: true,
		ReplyKeyboard:  scheduleReplyButtons(),
		IsPersistent:   true,
	}
)

func InlineScheduleKeyboard(groupID int) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}

	if groupID == 0 {
		return nil
	}

	inlineBtnDay := markup.Data("🔽", CurrentWeek, strconv.Itoa(groupID))
	markup.Inline(markup.Row(inlineBtnDay))

	return markup
}

func InlineWeekKeyboard(groupID int, weeks entities.Weeks) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var row []tele.Btn

	if !weeks.Prev.IsZero() {
		row = append(row, markup.Data("◀️", PrevWeek, fmt.Sprintf("%s:%s", strconv.Itoa(groupID), weeks.Prev.Format("02.01.2006"))))
	}
	if !weeks.Next.IsZero() {
		row = append(row, markup.Data("▶️", NextWeek, fmt.Sprintf("%s:%s", strconv.Itoa(groupID), weeks.Next.Format("02.01.2006"))))
	}

	if len(row) > 0 {
		markup.Inline(markup.Row(row...))
	}
	return markup
}

func scheduleReplyButtons() [][]tele.ReplyButton {
	return [][]tele.ReplyButton{
		{tele.ReplyButton{Text: BtnDay}, tele.ReplyButton{Text: BtnWeek}},
		{tele.ReplyButton{Text: BtnCalls}},
	}
}
