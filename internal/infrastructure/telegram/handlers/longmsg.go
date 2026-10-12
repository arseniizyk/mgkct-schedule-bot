package handlers

import (
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
)

func sendLongMessage(c tele.Context, text string, opt ...any) error {
	chunks := formatter.SplitMessage(text, 4096)
	if len(chunks) == 0 {
		return nil
	}

	for i, chunk := range chunks {
		// Клавиатуру прикрепляем только к последнему сообщению
		if i == len(chunks)-1 {
			if err := c.Send(chunk, opt...); err != nil {
				return err
			}
		} else {
			if err := c.Send(chunk, tele.ModeMarkdown); err != nil {
				return err
			}
		}
	}

	return nil
}

func editLongMessage(c tele.Context, text string, opt ...any) error {
	chunks := formatter.SplitMessage(text, 4096)
	if len(chunks) == 0 {
		return nil
	}

	if len(chunks) == 1 {
		return c.Edit(chunks[0], opt...)
	}

	if err := c.Delete(); err != nil {
		// Если не удалось удалить старое сообщение, просто редактируем первое
		return c.Edit(chunks[0], opt...)
	}

	return sendLongMessage(c, text, opt...)
}
