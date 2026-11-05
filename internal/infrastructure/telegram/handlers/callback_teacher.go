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
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/renderer"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

func handleTeacherWeekImage(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService) error {
	raw := dataFromCallbackData(data)
	left, dateStr, found := strings.CutLast(raw, ":")
	teacherName := raw
	if found {
		teacherName = left
	} else {
		dateStr = "current"
	}

	var teacher *entities.Teacher
	var err error
	if dateStr == "current" || dateStr == "" {
		teacher, err = svc.GetTeacherLatestSchedule(ctx, teacherName)
	} else {
		var d time.Time
		d, err = time.Parse("02.01.2006", dateStr)
		if err == nil {
			teacher, err = svc.GetTeacherScheduleByWeek(ctx, teacherName, d)
		}
	}
	if err != nil {
		return err
	}

	cacheKey := renderer.TeacherKey(teacher.Name, teacher.Week)
	cache := renderer.GetDefaultCache()
	if fileID, ok := cache.GetFileID(cacheKey); ok {
		return sendSchedulePhoto(c, &tele.Photo{FileID: fileID}, cache, cacheKey)
	}

	imgBytes, err := renderer.RenderTeacherWeek(teacher)
	if err != nil {
		return err
	}

	photo := &tele.Photo{
		File: tele.FromReader(bytes.NewReader(imgBytes)),
	}
	return sendSchedulePhoto(c, photo, cache, cacheKey)
}

func sendSchedulePhoto(c tele.Context, photo *tele.Photo, cache *renderer.ImageCache, cacheKey string) error {
	if c.Callback() != nil && c.Callback().IsInline() {
		sentMsg, err := c.Bot().Send(c.Sender(), photo)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{
				Text:      "Сначала запустите бота в личных сообщениях, чтобы получать расписание картинкой",
				ShowAlert: true,
			})
		}
		if sentMsg != nil && sentMsg.Photo != nil && sentMsg.Photo.FileID != "" && cache != nil {
			cache.SetFileID(cacheKey, sentMsg.Photo.FileID)
		}
		return c.Respond(&tele.CallbackResponse{
			Text: "🖼 Расписание отправлено вам в личные сообщения",
		})
	}

	_ = c.Respond()
	var (
		sentMsg *tele.Message
		err     error
	)
	if photo.FileID != "" {
		err = c.Send(photo)
	} else {
		sentMsg, err = c.Bot().Send(c.Recipient(), photo)
		if err == nil && sentMsg != nil && sentMsg.Photo != nil && sentMsg.Photo.FileID != "" && cache != nil {
			cache.SetFileID(cacheKey, sentMsg.Photo.FileID)
		}
	}
	return err
}

func parseTeacherDayNavData(raw string) (teacherName, dateStr string, dayIdx int, err error) {
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
		if possibleDate == "current" {
			return name, possibleDate, dayIdx, nil
		}
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
	if dateStr != "" && dateStr != "current" {
		date, parseErr := time.Parse("02.01.2006", dateStr)
		if parseErr != nil {
			return parseErr
		}
		teacher, err = svc.GetTeacherScheduleByWeek(ctx, teacherName, date)
		if err != nil {
			return err
		}
	} else {
		teacher, err = svc.GetTeacherLatestSchedule(ctx, teacherName)
		if err != nil {
			return err
		}
	}

	if dayIdx == -2 {
		dayIdx = formatter.EffectiveTeacherDayIndex(teacher)
	} else if dayIdx < 0 {
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

	if shouldShowTeacherProfile(ctx, userRepo, callbackChatID(c), teacherName) {
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
	if shouldShowTeacherProfile(ctx, userRepo, callbackChatID(c), teacherName) {
		member, _ := svc.GetFacultyMember(ctx, teacherName)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, teacher.Week)
	if err != nil {
		weeks = entities.WeekNavigation{Current: teacher.Week}
	}

	isInline := c.Callback() != nil && c.Callback().IsInline()
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(teacherName, weeks, isInline))
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
	if shouldShowTeacherProfile(ctx, userRepo, callbackChatID(c), teacherName) {
		member, _ := svc.GetFacultyMember(ctx, teacherName)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, date)
	if err != nil {
		weeks = entities.WeekNavigation{Current: date}
	}

	isInline := c.Callback() != nil && c.Callback().IsInline()
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(teacherName, weeks, isInline))
}
