package handlers

import (
	"strings"

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
		return safeEdit(c, chunks[0], opt...)
	}

	if err := c.Delete(); err != nil {
		// Если не удалось удалить старое сообщение, просто редактируем первое
		return safeEdit(c, chunks[0], opt...)
	}

	return sendLongMessage(c, text, opt...)
}

func safeEdit(c tele.Context, what any, opt ...any) error {
	var cleanOpts []any
	for _, o := range opt {
		if rm, ok := o.(*tele.ReplyMarkup); ok {
			// В c.Edit разрешен только InlineKeyboardMarkup.
			// Передача обычного ReplyKeyboardMarkup вызывает 400 Bad Request: inline keyboard expected.
			if rm != nil && len(rm.InlineKeyboard) > 0 {
				cleanOpts = append(cleanOpts, rm)
			}
			continue
		}
		cleanOpts = append(cleanOpts, o)
	}

	err := c.Edit(what, cleanOpts...)
	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil
	}
	return err
}
