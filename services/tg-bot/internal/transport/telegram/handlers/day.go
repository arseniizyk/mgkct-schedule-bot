package handlers

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	"gopkg.in/telebot.v4"
	tele "gopkg.in/telebot.v4"
)

func Day(log *slog.Logger, scheduleProvider ScheduleProvider) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With(
			"operation", "transport.telegram.handlers.Day",
			"chat_id", c.Chat().ID,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var (
			err      error
			schedule *pb.Group
		)

		// if command used like /day 99
		if len(c.Args()) > 0 {
			groupID, err := strconv.Atoi(c.Args()[0])
			if err != nil {
				log.Error("failed to parse user group input", "args", c.Args(), "error", err)
				return c.Send(messages.OnlyNumbers)
			}

			if schedule, err = scheduleProvider.GetGroupSchedule(ctx, groupID); err != nil {
				log.Error("failed to get group schedule", "group_id", groupID, "error", err)
				return c.Send(formatter.FormatErrorMessage(err))
			}
		} else {
			if schedule, err = scheduleProvider.GetGroupScheduleByChatID(ctx, c.Chat().ID); err != nil {
				log.Error("failed to get group schedule by id", "error", err)
				return c.Send(formatter.FormatErrorMessage(err))
			}
		}

		msg := formatter.FormatScheduleDay(schedule)

		return c.Send(msg, telebot.ModeMarkdown, keyboard.ReplyScheduleKeyboard, keyboard.InlineScheduleKeyboard(int(schedule.Id)))
	}
}
