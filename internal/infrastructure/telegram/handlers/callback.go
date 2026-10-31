package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

func CallbacksHandler(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	analyticsSvc AnalyticsService,
	userRepo UserRepo,
	sm StateManager,
	extra ...any,
) tele.HandlerFunc {
	var bot BotSender
	var adminIDs []int64
	for _, ex := range extra {
		switch v := ex.(type) {
		case BotSender:
			bot = v
		case []int64:
			adminIDs = v
		case context.Context:
			SetBroadcastContext(v)
		}
	}

	return func(c tele.Context) error {
		callback := c.Callback()
		data := callback.Data

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var err error
		switch {
		// Рассылка администратора
		case strings.Contains(data, keyboard.BroadcastConfirm), strings.Contains(data, keyboard.BroadcastCancel):
			return HandleBroadcastCallback(ctx, log, c, data, userRepo, bot, sm, adminIDs)

		// Выбор роли
		case strings.Contains(data, keyboard.RoleSelect):
			if sm != nil {
				err = handleRoleSelect(ctx, log, c, data, sm)
			}

		// Уведомления
		case strings.Contains(data, keyboard.NotifyToggle):
			if userRepo != nil {
				err = handleNotifyToggle(ctx, log, c, data, userRepo)
			}

		// Выбор подгруппы
		case strings.Contains(data, keyboard.SubgroupSelect):
			if userRepo != nil {
				err = handleSubgroupSelect(ctx, log, c, data, userRepo, sm)
			}

		// Кабинеты
		case strings.Contains(data, keyboard.CabinetPrevDay), strings.Contains(data, keyboard.CabinetNextDay):
			if analyticsSvc != nil {
				err = handleCabinetDayNav(ctx, log, c, data, analyticsSvc)
			}
		case strings.Contains(data, keyboard.CabinetCurrentWeek):
			if analyticsSvc != nil {
				err = handleCabinetWeek(ctx, log, c, data, analyticsSvc)
			}

		// Архив
		case strings.Contains(data, keyboard.ArchiveBack):
			err = handleArchiveBack(ctx, log, c, data, schedSvc, teacherSvc)
		case strings.Contains(data, keyboard.ArchiveSelect):
			err = handleArchiveSelect(ctx, log, c, data, schedSvc, teacherSvc, userRepo)

		// Закрепление преподавателя
		case strings.Contains(data, keyboard.TeacherSetSelect):
			if userRepo != nil {
				err = handleTeacherSetSelect(ctx, log, c, data, userRepo, sm)
			}

		// Преподаватели (ОБЯЗАТЕЛЬНО перед группами, чтобы t_currweek/t_prevday не попадали в обработчик групп!)
		case strings.Contains(data, keyboard.TeacherPrevDay), strings.Contains(data, keyboard.TeacherNextDay):
			err = handleTeacherDayNav(ctx, log, c, data, teacherSvc, userRepo)
		case strings.Contains(data, keyboard.TeacherCurrentWeek):
			err = handleTeacherCurrentWeek(ctx, log, c, data, teacherSvc, userRepo)
		case strings.Contains(data, keyboard.TeacherPrevWeek), strings.Contains(data, keyboard.TeacherNextWeek):
			err = handleTeacherWeekNav(ctx, log, c, data, teacherSvc, userRepo)

		// Группы
		case strings.Contains(data, keyboard.PrevDay), strings.Contains(data, keyboard.NextDay),
			strings.Contains(data, keyboard.DaySubgroup),
			strings.Contains(data, "prevday"), strings.Contains(data, "nextday"):
			err = handleDayNav(ctx, log, c, data, schedSvc, userRepo)
		case strings.Contains(data, keyboard.WeekSubgroup):
			err = handleWeekSubgroup(ctx, log, c, data, schedSvc, userRepo)
		case strings.Contains(data, keyboard.CurrentWeek), strings.Contains(data, "currentweek"):
			err = handleCurrentWeek(ctx, log, c, data, schedSvc, userRepo)
		case strings.Contains(data, keyboard.PrevWeek), strings.Contains(data, keyboard.NextWeek),
			strings.Contains(data, "prevweek"), strings.Contains(data, "nextweek"):
			err = handleWeekNav(ctx, log, c, data, schedSvc, userRepo)

		default:
			log.Warn("unexpected callback", "data", data)
			return c.Respond(&tele.CallbackResponse{Text: messages.Internal, ShowAlert: true})
		}

		if err != nil {
			if strings.Contains(err.Error(), "message is not modified") {
				return c.Respond()
			}
			log.Error("callback execution error", "error", err, "data", data)
			return c.Respond(&tele.CallbackResponse{Text: messages.Internal, ShowAlert: true})
		}

		return c.Respond()
	}
}

func handleRoleSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, sm StateManager) error {
	raw := dataFromCallbackData(data)
	chatID := c.Chat().ID

	// Очищаем инлайн-кнопки выбора роли у сообщения
	_ = c.Edit(&tele.ReplyMarkup{})

	switch raw {
	case keyboard.RoleStudent:
		if err := sm.Set(chatID, state.WaitingGroup); err != nil {
			log.Error("failed setting waiting group state from role select", "error", err)
			return c.Send(messages.Internal)
		}
		return c.Send("🎓 Вы выбрали роль: *Студент*\n\n"+messages.WaitingGroup, tele.ModeMarkdown)

	case keyboard.RoleTeacher:
		if err := sm.Set(chatID, state.WaitingTeacher); err != nil {
			log.Error("failed setting waiting teacher state from role select", "error", err)
			return c.Send(messages.Internal)
		}
		return c.Send("👨‍🏫 Вы выбрали роль: *Преподаватель*\n\n"+messages.WaitingTeacher, tele.ModeMarkdown)

	default:
		return fmt.Errorf("unknown role select data: %s", raw)
	}
}

func handleSubgroupSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, userRepo UserRepo, sm StateManager) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid subgroup select data: %s", raw)
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	subgroup, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	if err := userRepo.SetUserGroup(ctx, c.Chat().ID, groupID, subgroup); err != nil {
		log.Error("failed saving user group and subgroup", "group_id", groupID, "subgroup", subgroup, "error", err)
		return err
	}

	if sm != nil {
		_ = sm.Clear(c.Chat().ID)
	}

	var text string
	if subgroup > 0 {
		text = fmt.Sprintf(messages.GroupWithSubgroupSaved, groupID, subgroup)
	} else {
		text = messages.GroupSaved
	}

	_ = c.Edit(&tele.ReplyMarkup{})
	return c.Send(text, keyboard.ReplyScheduleKeyboard)
}

func handleTeacherSetSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, userRepo UserRepo, sm StateManager) error {
	raw := dataFromCallbackData(data)
	teacherName := raw
	if teacherName == "" {
		return fmt.Errorf("invalid teacher set select data: %s", data)
	}

	if err := userRepo.SetTeacher(ctx, c.Chat().ID, teacherName); err != nil {
		log.Error("failed saving user teacher from callback", "teacher", teacherName, "error", err)
		return err
	}

	if sm != nil {
		_ = sm.Clear(c.Chat().ID)
	}

	_ = c.Edit(&tele.ReplyMarkup{})
	return c.Send(messages.TeacherSaved, keyboard.ReplyScheduleKeyboard)
}

func parseGroupDayNavData(raw string) (groupID int, dateStr string, dayIdx int, subgroup int, hasSubgroup bool, err error) {
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return 0, "", 0, 0, false, fmt.Errorf("invalid day nav data: %s", raw)
	}

	groupID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", 0, 0, false, fmt.Errorf("invalid group id in %s: %w", raw, err)
	}

	switch len(parts) {
	case 2:
		dayIdx, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, "", 0, 0, false, err
		}
		return groupID, "", dayIdx, 0, false, nil

	case 3:
		if idx, err := strconv.Atoi(parts[1]); err == nil {
			sg, sgErr := strconv.Atoi(parts[2])
			if sgErr != nil {
				return 0, "", 0, 0, false, sgErr
			}
			return groupID, "", idx, sg, true, nil
		}
		dayIdx, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, "", 0, 0, false, err
		}
		return groupID, parts[1], dayIdx, 0, false, nil

	default: // >= 4
		dateStr = parts[1]
		dayIdx, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, "", 0, 0, false, err
		}
		sg, err := strconv.Atoi(parts[3])
		if err != nil {
			return 0, "", 0, 0, false, err
		}
		return groupID, dateStr, dayIdx, sg, true, nil
	}
}

func handleDayNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	groupID, dateStr, dayIdx, subgroup, hasSubgroup, err := parseGroupDayNavData(raw)
	if err != nil {
		return err
	}

	if !hasSubgroup && userRepo != nil {
		if userGroup, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, c.Chat().ID); err == nil && userGroup == groupID {
			subgroup = userSg
		}
	}

	var group *entities.Group
	if dateStr != "" {
		if date, parseErr := time.Parse("02.01.2006", dateStr); parseErr == nil {
			group, err = svc.GetGroupScheduleByWeek(ctx, int32(groupID), date)
		}
	}
	if group == nil {
		group, err = svc.GetGroupLatestSchedule(ctx, int32(groupID))
	}
	if err != nil {
		return err
	}

	if dayIdx < 0 {
		dayIdx = len(group.Days) - 1
	}
	if dayIdx < 0 {
		dayIdx = 0
	}
	if dayIdx >= len(group.Days) {
		dayIdx = len(group.Days) - 1
	}

	weeks, err := svc.GetAvailableWeeks(ctx, group.Week)
	if err != nil {
		weeks = entities.WeekNavigation{}
	}
	if weeks.Current.IsZero() {
		weeks.Current = group.Week
	}

	msg, err := formatter.FormatScheduleDayAtForSubgroup(group, dayIdx, subgroup)
	if err != nil {
		return err
	}

	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineDayKeyboard(groupID, dayIdx, len(group.Days), subgroup, weeks))
}

func handleWeekSubgroup(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid week subgroup data: %s", raw)
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	dateStr := parts[1]
	subgroup, err := strconv.Atoi(parts[2])
	if err != nil {
		return err
	}

	if dateStr == "current" {
		return renderCurrentWeek(ctx, c, svc, groupID, subgroup)
	}

	date, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return err
	}

	return renderWeek(ctx, c, svc, groupID, date, subgroup)
}

func handleCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	subgroup := 0
	if len(parts) >= 2 {
		if sg, err := strconv.Atoi(parts[1]); err == nil {
			subgroup = sg
		}
	} else if userRepo != nil {
		if userGroup, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, c.Chat().ID); err == nil && userGroup == groupID {
			subgroup = userSg
		}
	}

	return renderCurrentWeek(ctx, c, svc, groupID, subgroup)
}

func renderCurrentWeek(ctx context.Context, c tele.Context, svc ScheduleService, groupID, subgroup int) error {
	group, err := svc.GetGroupLatestSchedule(ctx, int32(groupID))
	if err != nil {
		return err
	}

	msg := formatter.FormatScheduleWeekForSubgroup(group, subgroup)
	weeks, err := svc.GetAvailableWeeks(ctx, group.Week)
	if err != nil {
		weeks = entities.WeekNavigation{}
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks, subgroup))
}

func handleWeekNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid week nav data: %s", raw)
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	date, err := time.Parse("02.01.2006", parts[1])
	if err != nil {
		return err
	}

	subgroup := 0
	if len(parts) >= 3 {
		if sg, err := strconv.Atoi(parts[2]); err == nil {
			subgroup = sg
		}
	} else if userRepo != nil {
		if userGroup, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, c.Chat().ID); err == nil && userGroup == groupID {
			subgroup = userSg
		}
	}

	return renderWeek(ctx, c, svc, groupID, date, subgroup)
}

func renderWeek(ctx context.Context, c tele.Context, svc ScheduleService, groupID int, date time.Time, subgroup int) error {
	group, err := svc.GetGroupScheduleByWeek(ctx, int32(groupID), date)
	if err != nil {
		return err
	}

	msg := formatter.FormatScheduleWeekForSubgroup(group, subgroup)
	weeks, err := svc.GetAvailableWeeks(ctx, date)
	if err != nil {
		weeks = entities.WeekNavigation{}
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks, subgroup))
}

func parseTeacherDayNavData(raw string) (teacherName string, dateStr string, dayIdx int, err error) {
	left, dayStr, found := strings.CutLast(raw, ":")
	if !found {
		return "", "", 0, fmt.Errorf("invalid teacher day nav data: %s", raw)
	}

	dayIdx, err = strconv.Atoi(dayStr)
	if err != nil {
		return "", "", 0, err
	}

	name, possibleDate, hasDate := strings.CutLast(left, ":")
	if hasDate {
		if _, parseErr := time.Parse("02.01.2006", possibleDate); parseErr == nil {
			return name, possibleDate, dayIdx, nil
		}
	}

	return left, "", dayIdx, nil
}

func handleTeacherDayNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	teacherName, dateStr, dayIdx, err := parseTeacherDayNavData(raw)
	if err != nil {
		return err
	}

	var teacher *entities.Teacher
	if dateStr != "" {
		if date, parseErr := time.Parse("02.01.2006", dateStr); parseErr == nil {
			teacher, err = svc.GetTeacherScheduleByWeek(ctx, teacherName, date)
		}
	}
	if teacher == nil {
		teacher, err = svc.GetTeacherLatestSchedule(ctx, teacherName)
	}
	if err != nil {
		return err
	}

	if dayIdx < 0 {
		dayIdx = len(teacher.Days) - 1
	}
	if dayIdx < 0 {
		dayIdx = 0
	}
	if dayIdx >= len(teacher.Days) {
		dayIdx = len(teacher.Days) - 1
	}

	weeks, err := svc.GetAvailableWeeks(ctx, teacher.Name, teacher.Week)
	if err != nil {
		weeks = entities.WeekNavigation{}
	}
	if weeks.Current.IsZero() {
		weeks.Current = teacher.Week
	}

	msg, err := formatter.FormatTeacherScheduleDayAt(teacher, dayIdx)
	if err != nil {
		return err
	}

	if shouldShowTeacherProfile(ctx, userRepo, c.Chat().ID, teacherName) {
		member, _ := svc.GetFacultyMember(ctx, teacherName)
		msg = formatter.FormatTeacherProfile(member) + msg
	}

	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherDayKeyboard(teacherName, dayIdx, len(teacher.Days), weeks))
}

func handleTeacherCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService, userRepo UserRepo) error {
	teacherName := dataFromCallbackData(data)

	teacher, err := svc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil {
		return err
	}

	var profile string
	if shouldShowTeacherProfile(ctx, userRepo, c.Chat().ID, teacherName) {
		member, _ := svc.GetFacultyMember(ctx, teacherName)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, teacher.Week)
	if err != nil {
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(teacherName, weeks))
}

func handleTeacherWeekNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	teacherName, dateStr, found := strings.CutLast(raw, ":")
	if !found {
		return fmt.Errorf("invalid teacher week nav data: %s", raw)
	}

	date, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return err
	}

	teacher, err := svc.GetTeacherScheduleByWeek(ctx, teacherName, date)
	if err != nil {
		return err
	}

	var profile string
	if shouldShowTeacherProfile(ctx, userRepo, c.Chat().ID, teacherName) {
		member, _ := svc.GetFacultyMember(ctx, teacherName)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, date)
	if err != nil {
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(teacherName, weeks))
}

func dataFromCallbackData(data string) string {
	parts := strings.Split(data, "|")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func handleCabinetDayNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc AnalyticsService) error {
	raw := dataFromCallbackData(data)
	cabinet, dayStr, found := strings.CutLast(raw, ":")
	if !found {
		return fmt.Errorf("invalid cabinet day nav data: %s", raw)
	}

	dayIdx, err := strconv.Atoi(dayStr)
	if err != nil {
		return err
	}

	sched, _, err := svc.GetCabinetSchedule(ctx, cabinet, time.Time{})
	if err != nil {
		return err
	}
	if sched == nil {
		return fmt.Errorf("cabinet schedule not found for %s", cabinet)
	}
	if dayIdx < 0 || dayIdx >= len(sched.Days) {
		return fmt.Errorf("day index %d out of range (total %d)", dayIdx, len(sched.Days))
	}

	msg := formatter.FormatCabinetDay(sched, dayIdx)
	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineCabinetDayKeyboard(cabinet, dayIdx, len(sched.Days)))
}

func handleCabinetWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc AnalyticsService) error {
	cabinet := dataFromCallbackData(data)
	sched, _, err := svc.GetCabinetSchedule(ctx, cabinet, time.Time{})
	if err != nil {
		return err
	}
	if sched == nil {
		return fmt.Errorf("cabinet schedule not found for %s", cabinet)
	}

	msg := formatter.FormatCabinetWeek(sched)
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}

func handleArchiveSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, schedSvc ScheduleService, teacherSvc TeacherService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	// формат "g:101:02.01.2006" или "t:Иванов И.И.:02.01.2006"
	parts := strings.Split(raw, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid archive select data: %s", raw)
	}

	targetType := parts[0]
	targetID := parts[1]
	dateStr := parts[2]

	date, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return err
	}

	kbd := keyboard.InlineArchiveReturnKeyboard(targetType, targetID)

	if targetType == "g" {
		groupID, err := strconv.Atoi(targetID)
		if err != nil {
			return err
		}
		group, err := schedSvc.GetArchiveSchedule(ctx, int32(groupID), date)
		if err != nil {
			return err
		}
		msg := formatter.FormatScheduleWeek(group)
		return editLongMessage(c, msg, tele.ModeMarkdown, kbd)
	}

	teacher, err := teacherSvc.GetTeacherArchiveSchedule(ctx, targetID, date)
	if err != nil {
		return err
	}
	var profile string
	if shouldShowTeacherProfile(ctx, userRepo, c.Chat().ID, targetID) {
		member, _ := teacherSvc.GetFacultyMember(ctx, targetID)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
	return editLongMessage(c, msg, tele.ModeMarkdown, kbd)
}

func handleArchiveBack(ctx context.Context, log *slog.Logger, c tele.Context, data string, schedSvc ScheduleService, teacherSvc TeacherService) error {
	raw := dataFromCallbackData(data)
	// формат "g:101" или "t:Иванов И.И."
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid archive back data: %s", raw)
	}

	targetType := parts[0]
	targetID := parts[1]

	if targetType == "g" {
		groupID, err := strconv.Atoi(targetID)
		if err != nil {
			return err
		}
		weeks, err := schedSvc.GetAllWeeks(ctx)
		if err != nil || len(weeks) == 0 {
			return safeEdit(c, fmt.Sprintf("Архив расписаний для группы %d пока пуст.", groupID))
		}
		msg := fmt.Sprintf("📦 *Архив расписания группы %d*\nВыберите неделю для просмотра:", groupID)
		kbd := keyboard.InlineArchiveWeeksKeyboard("g", targetID, weeks)
		return safeEdit(c, msg, tele.ModeMarkdown, kbd)
	}

	weeks, err := teacherSvc.GetAllWeeks(ctx, targetID)
	if err != nil || len(weeks) == 0 {
		return safeEdit(c, fmt.Sprintf("📦 Архив расписаний для преподавателя *%s* пока пуст.", targetID), tele.ModeMarkdown)
	}
	msg := fmt.Sprintf("📦 *Архив расписания: %s*\nВыберите неделю для просмотра:", targetID)
	kbd := keyboard.InlineArchiveWeeksKeyboard("t", targetID, weeks)
	return safeEdit(c, msg, tele.ModeMarkdown, kbd)
}
