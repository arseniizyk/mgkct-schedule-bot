package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/calendar"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

// ScheduleCalendar обрабатывает команду /calendar для скачивания .ics файла расписания
func ScheduleCalendar(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
	dict calendar.SubjectDictionary,
) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		args := c.Args()
		if len(args) > 0 {
			if groupID, err := strconv.Atoi(args[0]); err == nil && groupID > 0 {
				subgroup := 0
				if len(args) > 1 {
					if sg, err := strconv.Atoi(args[1]); err == nil {
						subgroup = sg
					}
				}
				return promptGroupCalendar(ctx, log, c, schedSvc, groupID, subgroup, dict)
			}

			teacherQuery := strings.Join(args, " ")
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
			if !ok {
				return sendTeacherNotFoundOrSuggestions(ctx, c, teacherSvc, teacherQuery, candidates)
			}
			return promptTeacherCalendar(ctx, log, c, teacherSvc, matched, dict)
		}

		groupID, subgroup, teacherName, err := userRepo.GetUserProfile(ctx, c.Chat().ID)
		if err != nil {
			return c.Send(formatter.FormatErrorMessage(err))
		}

		if teacherName != "" {
			return promptTeacherCalendar(ctx, log, c, teacherSvc, teacherName, dict)
		}

		if groupID == 0 {
			return c.Send(messages.UserNoGroup)
		}

		return promptGroupCalendar(ctx, log, c, schedSvc, groupID, subgroup, dict)
	}
}

func promptGroupCalendar(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	svc ScheduleService,
	groupID, subgroup int,
	dict calendar.SubjectDictionary,
) error {
	group, err := svc.GetGroupLatestSchedule(ctx, int32(groupID))
	if err != nil {
		log.Warn("failed getting group latest schedule for calendar export", "group_id", groupID, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	weeks, err := svc.GetAvailableWeeks(ctx, group.Week)
	if err != nil || (weeks.Next.IsZero() && weeks.Prev.IsZero()) {
		// Доступна только одна неделя — сразу отправляем файл
		return sendGroupCalendarFile(ctx, log, c, svc, groupID, subgroup, "current", dict)
	}

	var availableWeeks []time.Time
	availableWeeks = append(availableWeeks, group.Week)
	if !weeks.Next.IsZero() {
		availableWeeks = append(availableWeeks, weeks.Next)
	}

	kb := keyboard.InlineCalendarChoiceKeyboard("student", strconv.Itoa(groupID), subgroup, availableWeeks)
	text := fmt.Sprintf("📅 *Экспорт в календарь (.ics)*\nГруппа: *%d*\n\nВыберите неделю для выгрузки:", groupID)
	return c.Send(text, kb, tele.ModeMarkdown)
}

func promptTeacherCalendar(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	svc TeacherService,
	teacherName string,
	dict calendar.SubjectDictionary,
) error {
	teacher, err := svc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil {
		log.Warn("failed getting teacher latest schedule for calendar export", "teacher", teacherName, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, teacher.Week)
	if err != nil || (weeks.Next.IsZero() && weeks.Prev.IsZero()) {
		return sendTeacherCalendarFile(ctx, log, c, svc, teacherName, "current", dict)
	}

	var availableWeeks []time.Time
	availableWeeks = append(availableWeeks, teacher.Week)
	if !weeks.Next.IsZero() {
		availableWeeks = append(availableWeeks, weeks.Next)
	}

	kb := keyboard.InlineCalendarChoiceKeyboard("teacher", teacherName, 0, availableWeeks)
	text := fmt.Sprintf("📅 *Экспорт в календарь (.ics)*\nПреподаватель: *%s*\n\nВыберите неделю для выгрузки:", formatter.EscapeMD(teacherName))
	return c.Send(text, kb, tele.ModeMarkdown)
}

func sendGroupCalendarFile(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	svc ScheduleService,
	groupID, subgroup int,
	dateStr string,
	dict calendar.SubjectDictionary,
) error {
	var group *entities.Group
	var err error

	if dateStr == "current" || dateStr == "" {
		group, err = svc.GetGroupLatestSchedule(ctx, int32(groupID))
	} else {
		d, pErr := time.Parse("02.01.2006", dateStr)
		if pErr == nil {
			group, err = svc.GetGroupScheduleByWeek(ctx, int32(groupID), d)
		} else {
			err = pErr
		}
	}
	if err != nil {
		log.Warn("failed getting group schedule for calendar", "group_id", groupID, "error", err)
		if c.Callback() != nil && c.Callback().IsInline() {
			return c.Respond(&tele.CallbackResponse{
				Text:      formatter.FormatErrorMessage(err),
				ShowAlert: true,
			})
		}
		return c.Send(formatter.FormatErrorMessage(err))
	}

	icsData, err := calendar.GenerateGroupCalendar(group, subgroup, dict)
	if err != nil {
		log.Error("failed generating group calendar", "group_id", groupID, "error", err)
		if c.Callback() != nil && c.Callback().IsInline() {
			return c.Respond(&tele.CallbackResponse{
				Text:      messages.Internal,
				ShowAlert: true,
			})
		}
		return c.Send(messages.Internal)
	}

	filename := fmt.Sprintf("Расписание_Группа_%d_%s.ics", group.ID, group.Week.Format("02.01.2006"))
	caption := fmt.Sprintf("📅 Расписание группы %d на неделю %s\n\nОткройте этот файл, чтобы добавить пары в свой календарь.", group.ID, group.Week.Format("02.01.2006"))

	doc := &tele.Document{
		File:     tele.FromReader(bytes.NewReader(icsData)),
		FileName: filename,
		Caption:  caption,
		MIME:     "text/calendar",
	}

	if c.Callback() != nil && c.Callback().IsInline() {
		_, err := c.Bot().Send(c.Sender(), doc)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{
				Text:      "Сначала запустите бота в личных сообщениях, чтобы получать календарь",
				ShowAlert: true,
			})
		}
		return c.Respond(&tele.CallbackResponse{
			Text: "📅 Календарь отправлен вам в личные сообщения",
		})
	}

	return c.Send(doc)
}

func sendTeacherCalendarFile(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	svc TeacherService,
	teacherName string,
	dateStr string,
	dict calendar.SubjectDictionary,
) error {
	var teacher *entities.Teacher
	var err error

	if dateStr == "current" || dateStr == "" {
		teacher, err = svc.GetTeacherLatestSchedule(ctx, teacherName)
	} else {
		d, pErr := time.Parse("02.01.2006", dateStr)
		if pErr == nil {
			teacher, err = svc.GetTeacherScheduleByWeek(ctx, teacherName, d)
		} else {
			err = pErr
		}
	}
	if err != nil {
		log.Warn("failed getting teacher schedule for calendar", "teacher", teacherName, "error", err)
		if c.Callback() != nil && c.Callback().IsInline() {
			return c.Respond(&tele.CallbackResponse{
				Text:      formatter.FormatErrorMessage(err),
				ShowAlert: true,
			})
		}
		return c.Send(formatter.FormatErrorMessage(err))
	}

	icsData, err := calendar.GenerateTeacherCalendar(teacher, dict)
	if err != nil {
		log.Error("failed generating teacher calendar", "teacher", teacherName, "error", err)
		if c.Callback() != nil && c.Callback().IsInline() {
			return c.Respond(&tele.CallbackResponse{
				Text:      messages.Internal,
				ShowAlert: true,
			})
		}
		return c.Send(messages.Internal)
	}

	sanitizedName := strings.ReplaceAll(teacher.Name, " ", "_")
	filename := fmt.Sprintf("Расписание_%s_%s.ics", sanitizedName, teacher.Week.Format("02.01.2006"))
	caption := fmt.Sprintf("📅 Расписание преподавателя %s на неделю %s\n\nОткройте этот файл, чтобы добавить пары в свой календарь.", teacher.Name, teacher.Week.Format("02.01.2006"))

	doc := &tele.Document{
		File:     tele.FromReader(bytes.NewReader(icsData)),
		FileName: filename,
		Caption:  caption,
		MIME:     "text/calendar",
	}

	if c.Callback() != nil && c.Callback().IsInline() {
		_, err := c.Bot().Send(c.Sender(), doc)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{
				Text:      "Сначала запустите бота в личных сообщениях, чтобы получать календарь",
				ShowAlert: true,
			})
		}
		return c.Respond(&tele.CallbackResponse{
			Text: "📅 Календарь отправлен вам в личные сообщения",
		})
	}

	return c.Send(doc)
}

func handleWeekCalendar(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	data string,
	svc ScheduleService,
	userRepo UserRepo,
	dict calendar.SubjectDictionary,
) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 1 || parts[0] == "" {
		return fmt.Errorf("invalid week calendar data: %s", raw)
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	dateStr := "current"
	if len(parts) >= 2 && parts[1] != "" {
		dateStr = parts[1]
	}

	subgroup := 0
	if len(parts) >= 3 {
		if sg, err := strconv.Atoi(parts[2]); err == nil {
			subgroup = sg
		}
	} else if userRepo != nil {
		if userGroup, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, callbackChatID(c)); err == nil && userGroup == groupID {
			subgroup = userSg
		}
	}

	return sendGroupCalendarFile(ctx, log, c, svc, groupID, subgroup, dateStr, dict)
}

func handleTeacherWeekCalendar(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	data string,
	svc TeacherService,
	dict calendar.SubjectDictionary,
) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 1 || parts[0] == "" {
		return fmt.Errorf("invalid teacher week calendar data: %s", raw)
	}

	teacherName := parts[0]
	dateStr := "current"
	if len(parts) >= 2 && parts[1] != "" {
		dateStr = parts[1]
	}

	return sendTeacherCalendarFile(ctx, log, c, svc, teacherName, dateStr, dict)
}
