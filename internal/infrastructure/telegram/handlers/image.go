package handlers

import (
	"bytes"
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/renderer"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
)

// ScheduleImage обрабатывает команды /pic и /image для получения расписания недели в виде картинки
func ScheduleImage(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
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
				return sendGroupWeekImage(ctx, log, c, schedSvc, groupID, subgroup)
			}

			teacherName := strings.Join(args, " ")
			return sendTeacherWeekImage(ctx, log, c, teacherSvc, teacherName)
		}

		teacherName, err := userRepo.GetTeacher(ctx, c.Chat().ID)
		if err == nil && teacherName != "" {
			return sendTeacherWeekImage(ctx, log, c, teacherSvc, teacherName)
		}

		groupID, subgroup, err := userRepo.GroupAndSubgroupByChatID(ctx, c.Chat().ID)
		if err != nil {
			return c.Send(formatter.FormatErrorMessage(err))
		}

		return sendGroupWeekImage(ctx, log, c, schedSvc, groupID, subgroup)
	}
}

func sendGroupWeekImage(ctx context.Context, log *slog.Logger, c tele.Context, svc ScheduleService, groupID, subgroup int) error {
	group, err := svc.GetGroupLatestSchedule(ctx, int32(groupID))
	if err != nil {
		log.Warn("failed getting group schedule for image", "group_id", groupID, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	cacheKey := renderer.GroupKey(group.ID, group.Week, subgroup)
	cache := renderer.GetDefaultCache()
	if fileID, ok := cache.GetFileID(cacheKey); ok {
		photo := &tele.Photo{
			FileID: fileID,
		}
		return c.Send(photo)
	}

	imgBytes, err := renderer.RenderGroupWeek(group, subgroup)
	if err != nil {
		log.Error("failed rendering group week image", "group_id", groupID, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	photo := &tele.Photo{
		File: tele.FromReader(bytes.NewReader(imgBytes)),
	}
	sentMsg, err := c.Bot().Send(c.Recipient(), photo)
	if err != nil {
		log.Error("failed sending group week photo", "error", err)
		return err
	}

	if sentMsg != nil && sentMsg.Photo != nil && sentMsg.Photo.FileID != "" {
		cache.SetFileID(cacheKey, sentMsg.Photo.FileID)
	}

	return nil
}

func sendTeacherWeekImage(ctx context.Context, log *slog.Logger, c tele.Context, svc TeacherService, teacherName string) error {
	teacher, err := svc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil {
		log.Warn("failed getting teacher schedule for image", "name", teacherName, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	cacheKey := renderer.TeacherKey(teacher.Name, teacher.Week)
	cache := renderer.GetDefaultCache()
	if fileID, ok := cache.GetFileID(cacheKey); ok {
		photo := &tele.Photo{
			FileID: fileID,
		}
		return c.Send(photo)
	}

	imgBytes, err := renderer.RenderTeacherWeek(teacher)
	if err != nil {
		log.Error("failed rendering teacher week image", "name", teacherName, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	photo := &tele.Photo{
		File: tele.FromReader(bytes.NewReader(imgBytes)),
	}
	sentMsg, err := c.Bot().Send(c.Recipient(), photo)
	if err != nil {
		log.Error("failed sending teacher week photo", "error", err)
		return err
	}

	if sentMsg != nil && sentMsg.Photo != nil && sentMsg.Photo.FileID != "" {
		cache.SetFileID(cacheKey, sentMsg.Photo.FileID)
	}

	return nil
}
