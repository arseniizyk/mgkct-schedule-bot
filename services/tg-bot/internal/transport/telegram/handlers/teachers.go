package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
)

func Teachers(log *slog.Logger, teacherNamesProvider TeacherNamesProvider) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With("operation", "handlers.teachers")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		names, err := teacherNamesProvider.GetAllTeacherNames(ctx)
		if err != nil {
			log.Error("failed to get teacher names", "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		if len(names) == 0 {
			return c.Send("Список преподавателей пуст")
		}

		var sb strings.Builder
		sb.WriteString(messages.TeacherListHeader)
		sb.WriteString("\n")

		for i, name := range names {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, name)
		}

		return c.Send(sb.String(), tele.ModeMarkdown)
	}
}
