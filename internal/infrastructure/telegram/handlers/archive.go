package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

// Archive обрабатывает команду /archive [группа|преподаватель] [дата]
func Archive(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		args := c.Args()

		// 1. Без аргументов: смотрим сохраненный профиль пользователя
		if len(args) == 0 {
			groupID, err := userRepo.GroupByChatID(ctx, c.Chat().ID)
			if err == nil && groupID > 0 {
				weeks, err := schedSvc.GetAllWeeks(ctx)
				if err != nil || len(weeks) == 0 {
					return c.Send("📦 Архив расписаний для групп пока пуст.", keyboard.ReplyScheduleKeyboard)
				}
				msg := fmt.Sprintf("📦 *Архив расписания группы %d*\nВыберите неделю для просмотра:", groupID)
				kbd := keyboard.InlineArchiveWeeksKeyboard("g", strconv.Itoa(groupID), weeks)
				return c.Send(msg, tele.ModeMarkdown, kbd)
			}

			teacher, err := userRepo.GetTeacher(ctx, c.Chat().ID)
			if err == nil && teacher != "" {
				weeks, err := teacherSvc.GetAllWeeks(ctx, teacher)
				if err != nil || len(weeks) == 0 {
					return c.Send(fmt.Sprintf("📦 Архив расписаний для преподавателя *%s* пока пуст.", teacher), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
				}
				msg := fmt.Sprintf("📦 *Архив расписания: %s*\nВыберите неделю для просмотра:", teacher)
				kbd := keyboard.InlineArchiveWeeksKeyboard("t", teacher, weeks)
				return c.Send(msg, tele.ModeMarkdown, kbd)
			}

			return c.Send(
				"📦 *Архив расписания*\n\nПозволяет просматривать расписания прошедших недель колледжа.\n\n*Использование:*\n• `/archive <группа>` — выбор недели группы (напр. `/archive 101`)\n• `/archive <преподаватель>` — выбор недели преподавателя\n• `/archive <группа|преподаватель> <дата>` — сразу открыть дату (напр. `/archive 101 15.09.2025`)",
				tele.ModeMarkdown,
				keyboard.ReplyScheduleKeyboard,
			)
		}

		// 2. С аргументами: проверяем наличие даты в конце аргументов
		var targetDate time.Time
		var hasDate bool
		targetArgs := args

		if len(args) > 0 {
			if d, ok := parseArchiveDate(args[len(args)-1]); ok {
				targetDate = d
				hasDate = true
				targetArgs = args[:len(args)-1]
			}
		}

		// Если передан только один аргумент и это дата:
		if hasDate && len(targetArgs) == 0 {
			// Берем цель из профиля
			if groupID, err := userRepo.GroupByChatID(ctx, c.Chat().ID); err == nil && groupID > 0 {
				return sendGroupArchiveSchedule(ctx, c, schedSvc, int32(groupID), targetDate)
			}
			if teacher, err := userRepo.GetTeacher(ctx, c.Chat().ID); err == nil && teacher != "" {
				return sendTeacherArchiveSchedule(ctx, c, teacherSvc, teacher, targetDate)
			}
			return c.Send("Укажите группу или преподавателя для поиска в архиве: `/archive <группа> <дата>`", tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}

		targetStr := strings.Join(targetArgs, " ")

		// Проверяем, является ли цель группой (числом)
		if gID, err := strconv.Atoi(targetStr); err == nil && gID > 0 {
			if hasDate {
				return sendGroupArchiveSchedule(ctx, c, schedSvc, int32(gID), targetDate)
			}
			weeks, err := schedSvc.GetAllWeeks(ctx)
			if err != nil || len(weeks) == 0 {
				return c.Send(fmt.Sprintf("Архив расписаний для группы %d пока пуст.", gID), keyboard.ReplyScheduleKeyboard)
			}
			msg := fmt.Sprintf("📦 *Архив расписания группы %d*\nВыберите неделю:", gID)
			kbd := keyboard.InlineArchiveWeeksKeyboard("g", strconv.Itoa(gID), weeks)
			return c.Send(msg, tele.ModeMarkdown, kbd)
		}

		// Иначе пробуем как имя преподавателя
		validatedName, candidates, ok := teacherSvc.ValidateTeacher(ctx, targetStr)
		if !ok {
			if len(candidates) > 0 {
				var sb strings.Builder
				fmt.Fprintf(&sb, "По запросу *%s* найдено несколько преподавателей:\n\n", targetStr)
				for _, cand := range candidates {
					fmt.Fprintf(&sb, "• `/archive %s`\n", cand)
				}
				return c.Send(sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}
			return c.Send(fmt.Sprintf("Преподаватель или группа *%s* не найдены в архиве.", targetStr), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}

		if hasDate {
			return sendTeacherArchiveSchedule(ctx, c, teacherSvc, validatedName, targetDate)
		}

		weeks, err := teacherSvc.GetAllWeeks(ctx, validatedName)
		if err != nil || len(weeks) == 0 {
			return c.Send(fmt.Sprintf("Архив расписаний для преподавателя *%s* пока пуст.", validatedName), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}
		msg := fmt.Sprintf("📦 *Архив расписания: %s*\nВыберите неделю:", validatedName)
		kbd := keyboard.InlineArchiveWeeksKeyboard("t", validatedName, weeks)
		return c.Send(msg, tele.ModeMarkdown, kbd)
	}
}

func sendGroupArchiveSchedule(ctx context.Context, c tele.Context, svc ScheduleService, groupID int32, date time.Time) error {
	group, err := svc.GetArchiveSchedule(ctx, groupID, date)
	if err != nil {
		return c.Send(
			fmt.Sprintf("Расписание группы %d за дату *%s* не найдено в архиве.", groupID, date.Format("02.01.2006")),
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
		)
	}

	msg := formatter.FormatScheduleWeek(group)
	return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}

func sendTeacherArchiveSchedule(ctx context.Context, c tele.Context, svc TeacherService, teacherName string, date time.Time) error {
	teacher, err := svc.GetTeacherArchiveSchedule(ctx, teacherName, date)
	if err != nil {
		return c.Send(
			fmt.Sprintf("Расписание преподавателя *%s* за дату *%s* не найдено в архиве.", teacherName, date.Format("02.01.2006")),
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
		)
	}

	member, _ := svc.GetFacultyMember(ctx, teacherName)
	msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleWeek(teacher)
	return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}

func parseArchiveDate(s string) (time.Time, bool) {
	formats := []string{
		"02.01.2006",
		"2.1.2006",
		"2006-01-02",
		"02/01/2006",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
