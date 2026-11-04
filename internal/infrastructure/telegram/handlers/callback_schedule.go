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

func parseGroupDayNavData(raw string) (groupID int, dateStr string, dayIdx, subgroup int, hasSubgroup bool, err error) {
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
		if userGroup, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, callbackChatID(c)); err == nil && userGroup == groupID {
			subgroup = userSg
		}
	}

	var group *entities.Group
	if dateStr != "" && dateStr != "current" {
		date, parseErr := time.Parse("02.01.2006", dateStr)
		if parseErr != nil {
			return parseErr
		}
		group, err = svc.GetGroupScheduleByWeek(ctx, int32(groupID), date)
		if err != nil {
			return err
		}
	} else {
		group, err = svc.GetGroupLatestSchedule(ctx, int32(groupID))
		if err != nil {
			return err
		}
	}

	if dayIdx == -2 {
		filtered := formatter.FilterGroupForSubgroup(group, subgroup)
		dayIdx = formatter.EffectiveDayIndex(filtered)
	} else if dayIdx < 0 {
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
		if userGroup, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, callbackChatID(c)); err == nil && userGroup == groupID {
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

	isInline := c.Callback() != nil && c.Callback().IsInline()
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks, subgroup, isInline))
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
		if userGroup, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, callbackChatID(c)); err == nil && userGroup == groupID {
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

	isInline := c.Callback() != nil && c.Callback().IsInline()
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks, subgroup, isInline))
}

func handleWeekImage(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 1 || parts[0] == "" {
		return fmt.Errorf("invalid week image data: %s", raw)
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

	var group *entities.Group
	if dateStr == "current" {
		group, err = svc.GetGroupLatestSchedule(ctx, int32(groupID))
	} else {
		var d time.Time
		d, err = time.Parse("02.01.2006", dateStr)
		if err == nil {
			group, err = svc.GetGroupScheduleByWeek(ctx, int32(groupID), d)
		}
	}
	if err != nil {
		return err
	}

	cacheKey := renderer.GroupKey(group.ID, group.Week, subgroup)
	cache := renderer.GetDefaultCache()
	if fileID, ok := cache.GetFileID(cacheKey); ok {
		return sendSchedulePhoto(c, &tele.Photo{FileID: fileID}, cache, cacheKey)
	}

	imgBytes, err := renderer.RenderGroupWeek(group, subgroup)
	if err != nil {
		return err
	}

	photo := &tele.Photo{
		File: tele.FromReader(bytes.NewReader(imgBytes)),
	}
	return sendSchedulePhoto(c, photo, cache, cacheKey)
}
