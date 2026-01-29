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
	tele "gopkg.in/telebot.v4"
)

func Week(log *slog.Logger, scheduleProvider ScheduleProvider, weekProvider WeekProvider) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With(
			"operation", "transport.telegram.handlers.Week",
			"chat_id", c.Chat().ID,
		)

		var (
			schedule *pb.Group
			err      error
		)

		// if command used like: /week 111
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if len(c.Args()) > 0 {
			groupID, err := strconv.Atoi(c.Args()[0])
			if err != nil {
				log.Error("failed to parse user group input", "args", c.Args(), "error", err)
				return c.Send(messages.OnlyNumbers)
			}

			if schedule, err = scheduleProvider.GetGroupSchedule(ctx, groupID); err != nil {
				log.Error("failed to get group schedule", "group_id", groupID, "error", err)
				return c.Send(formatter.FormatTransportError(err), tele.ModeMarkdown)
			}
		} else {
			schedule, err = scheduleProvider.GetGroupScheduleByChatID(ctx, c.Chat().ID)
			if err != nil {
				log.Error("failed to get group schedule by id", "error", err)
				return c.Send(formatter.FormatErrorMessage(err), tele.ModeMarkdown)
			}
		}

		msg := formatter.FormatScheduleWeek(schedule)

		weeks, err := weekProvider.GetAvailableWeeks(ctx, nil)
		if err != nil {
			log.Error("failed to get available weeks")
			return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}

		return c.Send(msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(int(schedule.Id), weeks))
	}
}
