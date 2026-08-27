package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/state"
)

func SetTeacher(log *slog.Logger, teacherSaver TeacherSaver, teacherValidator TeacherValidator, stateSetter StateSetter) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With("operation", "handlers.setteacher")

		args := c.Args()

		if len(args) == 0 {
			if err := stateSetter.Set(c.Chat().ID, state.WaitingTeacher); err != nil {
				log.Error("failed to set state", "error", err)
				return c.Send(messages.Internal)
			}
			return c.Send(messages.WaitingTeacher)
		}

		teacherName := strings.Join(args, " ")

		matchedName, candidates, ok := teacherValidator.ValidateTeacher(context.Background(), teacherName)
		if !ok {
			if len(candidates) > 0 {
				return c.Send(fmt.Sprintf(messages.TeacherAmbiguous, formatCandidates(candidates)))
			}
			return c.Send(messages.TeacherNotFound)
		}

		if err := teacherSaver.SetTeacher(context.Background(), c.Chat().ID, matchedName); err != nil {
			log.Error("failed to save teacher", "error", err)
			return c.Send(messages.Internal)
		}

		return c.Send(messages.TeacherSaved, keyboard.ReplyScheduleKeyboard)
	}
}
