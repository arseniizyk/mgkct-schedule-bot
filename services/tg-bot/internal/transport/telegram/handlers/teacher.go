package handlers

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
)

func TeacherDay(log *slog.Logger, teacherScheduleProvider TeacherScheduleProvider) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With("operation", "handlers.teacherday")

		args := c.Args()

		var teacher *pb.Teacher
		var err error

		if len(args) > 0 {
			teacherName := args[0]
			teacher, err = teacherScheduleProvider.GetTeacherSchedule(context.Background(), teacherName)
		} else {
			teacher, err = teacherScheduleProvider.GetTeacherScheduleByChatID(context.Background(), c.Chat().ID)
		}

		if err != nil {
			log.Error("failed to get teacher schedule", "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		dayIdx := formatter.EffectiveTeacherDayIndex(teacher)
		text, err := formatter.FormatTeacherScheduleDayAt(teacher, dayIdx)
		if err != nil {
			return c.Send(messages.Internal)
		}

		return c.Send(text, tele.ModeMarkdown, keyboard.InlineTeacherDayKeyboard(c.Chat().ID, dayIdx, len(teacher.GetDays())))
	}
}

func TeacherWeek(log *slog.Logger, teacherScheduleProvider TeacherScheduleProvider, teacherWeekProvider TeacherWeekProvider) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With("operation", "handlers.teacherweek")

		args := c.Args()

		var teacher *pb.Teacher
		var err error

		if len(args) > 0 {
			teacherName := args[0]
			teacher, err = teacherScheduleProvider.GetTeacherSchedule(context.Background(), teacherName)
		} else {
			teacher, err = teacherScheduleProvider.GetTeacherScheduleByChatID(context.Background(), c.Chat().ID)
		}

		if err != nil {
			log.Error("failed to get teacher schedule", "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		text := formatter.FormatTeacherScheduleWeek(teacher)

		weeks, err := teacherWeekProvider.GetAvailableTeacherWeeks(context.Background(), nil)
		if err != nil {
			return sendLongMessage(c, text, tele.ModeMarkdown)
		}

		entitiesWeeks := convertToEntitiesWeeks(weeks)
		return sendLongMessage(c, text, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(c.Chat().ID, entitiesWeeks))
	}
}

func convertToEntitiesWeeks(weeks *pb.AvailableWeeksResponse) entities.Weeks {
	result := entities.Weeks{}

	if weeks.GetPrev() != nil {
		result.Prev = weeks.GetPrev().AsTime()
	}
	if weeks.GetCurrent() != nil {
		result.Current = weeks.GetCurrent().AsTime()
	}
	if weeks.GetNext() != nil {
		result.Next = weeks.GetNext().AsTime()
	}

	return result
}
