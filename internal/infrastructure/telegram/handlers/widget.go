package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/widget"
)

// MTProtoSender defines the interface for delivering native MTProto widgets.
type MTProtoSender interface {
	IsReady() bool
	SendWidget(ctx context.Context, chatID int64, html, caption string) error
}

// Widget handles the /widget command to deliver or preview an interactive Mini-HTML schedule widget.
func Widget(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
	webAppURL string,
	mtprotoSender MTProtoSender,
) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()

		if c.Chat() != nil && (c.Chat().Type == tele.ChatGroup || c.Chat().Type == tele.ChatSuperGroup || c.Chat().Type == tele.ChatChannel) {
			return c.Send(
				"⚠️ *Интерактивные виджеты поддерживаются Telegram только в личных сообщениях с ботом.*\n\nВ групповых чатах и каналах отправка нативных виджетов ограничена самой платформой Telegram. Пожалуйста, откройте личные сообщения с ботом и вызовите команду там.",
				tele.ModeMarkdown,
			)
		}

		args := c.Args()
		if len(args) > 0 {
			// Если аргумент число — расписание группы
			groupID, err := strconv.Atoi(args[0])
			if err == nil && groupID > 0 {
				subgroup := 0
				if len(args) > 1 {
					if sg, sgErr := strconv.Atoi(args[1]); sgErr == nil && (sg == 1 || sg == 2) {
						subgroup = sg
					}
				}
				return sendGroupWidget(ctx, log, c, schedSvc, groupID, subgroup, webAppURL, mtprotoSender)
			}

			// Иначе пробуем найти преподавателя по фамилии
			query := strings.Join(args, " ")
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
			if !ok {
				return sendTeacherNotFoundOrSuggestions(ctx, c, teacherSvc, query, candidates)
			}

			return sendTeacherWidget(ctx, log, c, teacherSvc, matched, webAppURL, mtprotoSender)
		}

		// Без аргументов: определяем сохраненный профиль пользователя
		groupID, subgroup, teacherName, err := userRepo.GetUserProfile(ctx, c.Chat().ID)
		if err != nil {
			return c.Send(formatter.FormatErrorMessage(err))
		}

		if teacherName != "" {
			return sendTeacherWidget(ctx, log, c, teacherSvc, teacherName, webAppURL, mtprotoSender)
		}

		return sendGroupWidget(ctx, log, c, schedSvc, groupID, subgroup, webAppURL, mtprotoSender)
	}
}

func sendGroupWidget(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	schedSvc ScheduleService,
	groupID int,
	subgroup int,
	webAppURL string,
	mtprotoSender MTProtoSender,
) error {
	const op = "handlers.sendGroupWidget"

	group, err := schedSvc.GetGroupLatestSchedule(ctx, int32(groupID))
	if err != nil {
		log.Warn("failed getting group schedule for widget", "operation", op, "group_id", groupID, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	html, err := widget.BuildGroupWidget(group)
	if err != nil {
		log.Error("failed building group widget", "operation", op, "group_id", groupID, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	filtered := formatter.FilterGroupForSubgroup(group, subgroup)
	dayIdx := formatter.EffectiveDayIndex(filtered)
	weeks, _ := schedSvc.GetAvailableWeeks(ctx, group.Week)
	if weeks.Current.IsZero() {
		weeks.Current = group.Week
	}

	fallbackMsg := fmt.Sprintf("📱 *Интерактивный виджет расписания (Группа %d)*\n\n%s", group.ID, formatter.FormatScheduleDay(filtered))

	// 1. Попытка нативной отправки через MTProto Layer 233 (без текста под виджетом)
	if mtprotoSender != nil && mtprotoSender.IsReady() && c.Chat() != nil {
		if mtErr := mtprotoSender.SendWidget(ctx, c.Chat().ID, html, ""); mtErr == nil {
			log.Info("widget sent natively via MTProto", "operation", op, "chat_id", c.Chat().ID)
			return nil
		} else {
			log.Warn("MTProto send failed, attempting Bot API fallback", "operation", op, "error", mtErr)
		}
	}

	// 2. Фолбэк на Bot API
	var markup *tele.ReplyMarkup
	if webAppURL != "" {
		previewURL := fmt.Sprintf("%s/widget/preview?group=%d", strings.TrimRight(webAppURL, "/"), group.ID)
		markup = &tele.ReplyMarkup{}
		previewBtn := markup.WebApp("🌐 Открыть интерактивный виджет", &tele.WebApp{URL: previewURL})
		weekBtn := markup.Data("📆 Неделя", keyboard.CurrentWeek, strconv.Itoa(int(group.ID)))
		markup.Inline(markup.Row(previewBtn), markup.Row(weekBtn))
	} else {
		markup = keyboard.InlineDayKeyboard(int(group.ID), dayIdx, len(group.Days), subgroup, weeks)
	}

	return widget.SendWidgetOrFallback(ctx, log, c, html, fallbackMsg, markup)
}

func sendTeacherWidget(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	teacherSvc TeacherService,
	teacherName string,
	webAppURL string,
	mtprotoSender MTProtoSender,
) error {
	const op = "handlers.sendTeacherWidget"

	teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil {
		log.Warn("failed getting teacher schedule for widget", "operation", op, "name", teacherName, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	html, err := widget.BuildTeacherWidget(teacher)
	if err != nil {
		log.Error("failed building teacher widget", "operation", op, "name", teacherName, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	dayIdx := formatter.EffectiveTeacherDayIndex(teacher)
	weeks, _ := teacherSvc.GetAvailableWeeks(ctx, teacher.Name, teacher.Week)
	if weeks.Current.IsZero() {
		weeks.Current = teacher.Week
	}

	fallbackMsg := fmt.Sprintf("📱 *Интерактивный виджет расписания (%s)*\n\n%s", teacher.Name, formatter.FormatTeacherScheduleDay(teacher))

	// 1. Попытка нативной отправки через MTProto Layer 233 (без текста под виджетом)
	if mtprotoSender != nil && mtprotoSender.IsReady() && c.Chat() != nil {
		if mtErr := mtprotoSender.SendWidget(ctx, c.Chat().ID, html, ""); mtErr == nil {
			log.Info("widget sent natively via MTProto", "operation", op, "chat_id", c.Chat().ID)
			return nil
		} else {
			log.Warn("MTProto send failed, attempting Bot API fallback", "operation", op, "error", mtErr)
		}
	}

	// 2. Фолбэк на Bot API
	var markup *tele.ReplyMarkup
	if webAppURL != "" {
		encodedTeacher := url.QueryEscape(teacher.Name)
		previewURL := fmt.Sprintf("%s/widget/preview?teacher=%s", strings.TrimRight(webAppURL, "/"), encodedTeacher)
		markup = &tele.ReplyMarkup{}
		previewBtn := markup.WebApp("🌐 Открыть интерактивный виджет", &tele.WebApp{URL: previewURL})
		weekBtn := markup.Data("📆 Неделя", keyboard.TeacherCurrentWeek, teacher.Name)
		markup.Inline(markup.Row(previewBtn), markup.Row(weekBtn))
	} else {
		markup = keyboard.InlineTeacherDayKeyboard(teacher.Name, dayIdx, len(teacher.Days), weeks)
	}

	return widget.SendWidgetOrFallback(ctx, log, c, html, fallbackMsg, markup)
}
