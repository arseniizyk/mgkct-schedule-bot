package keyboard

import (
	"strconv"

	tele "gopkg.in/telebot.v4"
)

const (
	BtnDay   = "📅 День"
	BtnWeek  = "📆 Неделя"
	BtnCalls = "⏰ Звонки"
	Week     = "week"
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

	InlineBtnDay := markup.Data("🔽", Week, strconv.Itoa(groupID))
	markup.Inline(markup.Row(InlineBtnDay))

	return markup
}

func scheduleReplyButtons() [][]tele.ReplyButton {
	return [][]tele.ReplyButton{
		{tele.ReplyButton{Text: BtnDay}, tele.ReplyButton{Text: BtnWeek}},
		{tele.ReplyButton{Text: BtnCalls}},
	}
}
