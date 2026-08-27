package handlers

import (
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
)

func sendLongMessage(c tele.Context, msg string, options ...any) error {
	chunks := formatter.SplitMessage(msg, 4096)
	if len(chunks) == 0 {
		return nil
	}

	for i, chunk := range chunks {
		if i == 0 {
			if err := c.Send(chunk, options...); err != nil {
				return err
			}
			continue
		}
		if err := c.Send(chunk); err != nil {
			return err
		}
	}

	return nil
}

func editLongMessage(c tele.Context, msg string, options ...any) error {
	chunks := formatter.SplitMessage(msg, 4096)
	if len(chunks) == 0 {
		return nil
	}

	if err := c.Edit(chunks[0], options...); err != nil {
		return err
	}

	for _, chunk := range chunks[1:] {
		if err := c.Send(chunk); err != nil {
			return err
		}
	}

	return nil
}
