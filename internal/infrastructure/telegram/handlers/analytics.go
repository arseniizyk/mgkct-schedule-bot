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
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

// Cabinet обрабатывает команду /cabinet <номер>
func Cabinet(log *slog.Logger, analyticsSvc AnalyticsService) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			return c.Send(
				"🚪 *Поиск расписания по аудитории*\n\nИспользование: `/cabinet <номер>`\nНапример: `/cabinet 301` или `/cabinet 3-207`\n\nСписок всех кабинетов: /cabinets",
				tele.ModeMarkdown,
				keyboard.ReplyScheduleKeyboard,
			)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := strings.Join(args, " ")
		cabSched, candidates, err := analyticsSvc.GetCabinetSchedule(ctx, query, time.Time{})
		if err != nil {
			log.Warn("failed getting cabinet schedule", "query", query, "error", err)
			return c.Send("Ошибка при получении расписания кабинета.", keyboard.ReplyScheduleKeyboard)
		}

		if cabSched == nil {
			if len(candidates) > 0 {
				var sb strings.Builder
				fmt.Fprintf(&sb, "По запросу *%s* найдено несколько кабинетов:\n\n", query)
				for _, cand := range candidates {
					fmt.Fprintf(&sb, "• `/cabinet %s`\n", cand)
				}
				return c.Send(sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}
			return c.Send(fmt.Sprintf("Кабинет *%s* не найден в расписании.", query), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}

		dayIdx := service.EffectiveEndingsDayIndex()
		if dayIdx >= len(cabSched.Days) {
			dayIdx = 0
		}

		msg := formatter.FormatCabinetDay(cabSched, dayIdx)
		return c.Send(
			msg,
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
			keyboard.InlineCabinetDayKeyboard(cabSched.Cabinet, dayIdx, len(cabSched.Days)),
		)
	}
}

// Cabinets обрабатывает команду /cabinets (список всех аудиторий)
func Cabinets(log *slog.Logger, analyticsSvc AnalyticsService) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cabinets, err := analyticsSvc.GetAllCabinets(ctx)
		if err != nil {
			log.Error("failed getting cabinets list", "error", err)
			return c.Send("Не удалось получить список кабинетов.", keyboard.ReplyScheduleKeyboard)
		}

		if len(cabinets) == 0 {
			return c.Send("Кабинеты в расписании не найдены.", keyboard.ReplyScheduleKeyboard)
		}

		var sb strings.Builder
		sb.WriteString("🚪 *Аудитории колледжа в расписании:*\n\n")

		lineCount := 0
		for _, cab := range cabinets {
			fmt.Fprintf(&sb, "`/cabinet %s`  ", cab)
			lineCount++
			if lineCount%3 == 0 {
				sb.WriteString("\n")
			}
		}

		return sendLongMessage(c, sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}
}

// Endings обрабатывает команду /endings (аналитика окончания пар)
func Endings(log *slog.Logger, analyticsSvc AnalyticsService) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		dayIdx := service.EffectiveEndingsDayIndex()
		args := c.Args()
		if len(args) > 0 {
			if parsed, err := strconv.Atoi(args[0]); err == nil && parsed >= 1 && parsed <= 6 {
				dayIdx = parsed - 1
			}
		}

		stat, err := analyticsSvc.GetEndings(ctx, dayIdx)
		if err != nil {
			log.Error("failed getting endings stat", "error", err)
			return c.Send("Не удалось получить аналитику окончания пар.", keyboard.ReplyScheduleKeyboard)
		}

		msg := formatter.FormatEndings(stat)
		return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}
}

// Stats обрабатывает команду /stats [группа/преподаватель]
func Stats(log *slog.Logger, analyticsSvc AnalyticsService, userRepo UserRepo, teacherSvc TeacherService) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		args := c.Args()
		if len(args) > 0 {
			query := strings.Join(args, " ")
			// Проверяем, число ли это (номер группы)
			if grpID, err := strconv.Atoi(query); err == nil && grpID > 0 {
				stats, total, err := analyticsSvc.GetGroupStats(ctx, int32(grpID), time.Time{})
				if err != nil {
					return c.Send(fmt.Sprintf("Расписание группы %d не найдено.", grpID), keyboard.ReplyScheduleKeyboard)
				}
				msg := formatter.FormatStats(fmt.Sprintf("Группа %d", grpID), stats, total)
				return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}

			// Проверяем, имя ли это преподавателя
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
			if ok {
				stats, total, err := analyticsSvc.GetTeacherStats(ctx, matched, time.Time{})
				if err != nil {
					return c.Send(fmt.Sprintf("Расписание преподавателя %s не найдено.", matched), keyboard.ReplyScheduleKeyboard)
				}
				msg := formatter.FormatStats(matched, stats, total)
				return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}
			if len(candidates) > 0 {
				return c.Send(fmt.Sprintf("Уточните запрос:\n%s", formatCandidates(candidates)))
			}
		}

		// Если без аргументов — смотрим сохраненную группу или преподавателя пользователя
		grpID, _ := userRepo.GroupByChatID(ctx, c.Chat().ID)
		if grpID > 0 {
			stats, total, err := analyticsSvc.GetGroupStats(ctx, int32(grpID), time.Time{})
			if err == nil {
				msg := formatter.FormatStats(fmt.Sprintf("Группа %d (ваша)", grpID), stats, total)
				return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}
		}

		teacherName, _ := userRepo.GetTeacher(ctx, c.Chat().ID)
		if teacherName != "" {
			stats, total, err := analyticsSvc.GetTeacherStats(ctx, teacherName, time.Time{})
			if err == nil {
				msg := formatter.FormatStats(fmt.Sprintf("%s (ваше)", teacherName), stats, total)
				return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}
		}

		return c.Send(
			"📊 *Статистика нагрузки пар*\n\nИспользование:\n`/stats <номер группы>` — например, `/stats 101`\n`/stats <фамилия>` — например, `/stats Иванов`",
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
		)
	}
}
