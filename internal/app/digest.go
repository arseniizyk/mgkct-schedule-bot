package app

import (
	"context"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

type groupSubgroupKey struct {
	groupID  int
	subgroup int
}

func (a *App) isGroupDayEnded(ctx context.Context, groupID, subgroup, dayIdx int, now time.Time) bool {
	grp, err := a.scheduleSvc.GetGroupLatestSchedule(ctx, int32(groupID))
	if err != nil || dayIdx >= len(grp.Days) {
		return false
	}

	filtered := formatter.FilterGroupForSubgroup(grp, subgroup)
	if dayIdx >= len(filtered.Days) || filtered.Days[dayIdx] == nil {
		return false
	}

	lastSubject := formatter.FindLastSubject(filtered.Days[dayIdx].Subjects)
	if lastSubject == -1 {
		return false
	}

	endTime, ok := formatter.GetDayEndTime(dayIdx, lastSubject+1, timeutil.MinskLocation)
	if !ok || now.Before(endTime) {
		return false
	}

	return true
}

func (a *App) isTeacherDayEnded(ctx context.Context, teacherName string, dayIdx int, now time.Time) bool {
	teacher, err := a.teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil || dayIdx >= len(teacher.Days) {
		return false
	}

	lastSubject := formatter.FindLastSubject(teacher.Days[dayIdx].Subjects)
	if lastSubject == -1 {
		return false
	}

	endTime, ok := formatter.GetDayEndTime(dayIdx, lastSubject+1, timeutil.MinskLocation)
	if !ok || now.Before(endTime) {
		return false
	}

	return true
}

func (a *App) seedDigestLogIfNeeded(ctx context.Context) {
	log := a.log.With("operation", "app.App.seedDigestLogIfNeeded")

	now := timeutil.Now()
	weekday := now.Weekday()

	// В воскресенье до 20:00 рассылка не происходит — сидировать нечего.
	if weekday == time.Sunday && now.Hour() < 20 {
		return
	}

	todayStr := timeutil.TodayStr()

	// Оптимальный запрос: получаем только уникальные пары (группа, подгруппа)
	// и преподавателей подписчиков без выгрузки всех пользователей в память.
	groupTargets, err := a.userRepo.DistinctSubscribedGroups(ctx)
	if err != nil {
		log.Warn("failed fetching subscribed groups for seeding", "error", err)
	}
	teachers, err := a.userRepo.DistinctSubscribedTeachers(ctx)
	if err != nil {
		log.Warn("failed fetching subscribed teachers for seeding", "error", err)
	}

	if len(groupTargets) == 0 && len(teachers) == 0 {
		return
	}

	seeded := 0

	// Если бот запущен после 20:00, помечаем все группы и преподавателей как отправленные,
	// чтобы при перезапуске ночью не разослать дайджест повторно.
	if now.Hour() >= 20 {
		for _, target := range groupTargets {
			if err := a.digestRepo.MarkGroupDigestSent(ctx, target.GroupID, target.Subgroup, todayStr); err != nil {
				log.Warn("failed seeding group digest log", "group", target.GroupID, "subgroup", target.Subgroup, "error", err)
				continue
			}
			seeded++
		}
		for _, teacherName := range teachers {
			if err := a.digestRepo.MarkTeacherDigestSent(ctx, teacherName, todayStr); err != nil {
				log.Warn("failed seeding teacher digest log", "teacher", teacherName, "error", err)
				continue
			}
			seeded++
		}
		if seeded > 0 {
			log.Info("seeded digest log on startup after 20:00 to prevent re-broadcast", "seeded_count", seeded, "date", todayStr)
		}
		return
	}

	dayIdx := int(weekday - 1) // 0..5 для Пн..Сб

	for _, target := range groupTargets {
		if !a.isGroupDayEnded(ctx, target.GroupID, target.Subgroup, dayIdx, now) {
			continue
		}

		if err := a.digestRepo.MarkGroupDigestSent(ctx, target.GroupID, target.Subgroup, todayStr); err != nil {
			log.Warn("failed seeding group digest log", "group", target.GroupID, "subgroup", target.Subgroup, "error", err)
			continue
		}
		seeded++
	}

	for _, teacherName := range teachers {
		if !a.isTeacherDayEnded(ctx, teacherName, dayIdx, now) {
			continue
		}

		if err := a.digestRepo.MarkTeacherDigestSent(ctx, teacherName, todayStr); err != nil {
			log.Warn("failed seeding teacher digest log", "teacher", teacherName, "error", err)
			continue
		}
		seeded++
	}

	if seeded > 0 {
		log.Info("seeded digest log on startup for already ended lessons", "seeded_count", seeded, "date", todayStr)
	}
}

func (a *App) startEveningDigestScheduler(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			localTime := timeutil.Now()
			weekday := localTime.Weekday()

			// В воскресенье отправляем дайджест на понедельник строго в 20:00 (фоллбэк)
			if weekday == time.Sunday {
				if localTime.Hour() >= 20 {
					todayStr := timeutil.TodayStr()
					a.sendEveningDigestFallback(ctx, todayStr)
				}
				continue
			}

			// Будние дни и суббота: Пн=0..Сб=5
			dayIdx := int(weekday - 1)
			todayStr := timeutil.TodayStr()

			// Основная логика: отправка дайджеста по окончании пар
			// Ограничиваем окно проверки: с 12:00 до 21:00
			if localTime.Hour() >= 12 && localTime.Hour() < 21 {
				users, err := a.notifier.UsersForNotification(ctx)
				if err == nil && len(users) > 0 {
					targets, teachers := extractUniqueRecipients(users)
					a.notifyGroupsAfterLessons(ctx, localTime, dayIdx, targets, todayStr, users)
					a.notifyTeachersAfterLessons(ctx, localTime, dayIdx, teachers, todayStr, users)
				}
			}

			// Фоллбэк в 20:00: отправляем всем, кому дайджест ещё не ушёл
			if localTime.Hour() >= 20 {
				a.sendEveningDigestFallback(ctx, todayStr)
			}
		}
	}
}

func (a *App) sendEveningDigestFallback(ctx context.Context, todayStr string) {
	users, err := a.notifier.UsersForNotification(ctx)
	if err != nil || len(users) == 0 {
		return
	}

	targets, teachers := extractUniqueRecipients(users)

	for target := range targets {
		sent, err := a.digestRepo.IsGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr)
		if err != nil {
			a.log.Warn("failed checking group digest sent status for fallback", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}
		if sent {
			continue
		}

		if err := a.digestRepo.MarkGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr); err != nil {
			a.log.Warn("failed marking group digest as sent in fallback", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}

		a.log.Info("Sending 20:00 fallback digest for group/subgroup", "group", target.groupID, "subgroup", target.subgroup)
		if _, sendErr := a.notifier.SendNextDayDigestForGroupSubgroup(ctx, a.scheduleSvc, target.groupID, target.subgroup, users); sendErr != nil {
			a.log.Warn("failed sending 20:00 fallback digest for group/subgroup", "group", target.groupID, "subgroup", target.subgroup, "error", sendErr)
		}
	}

	for teacherName := range teachers {
		sent, err := a.digestRepo.IsTeacherDigestSent(ctx, teacherName, todayStr)
		if err != nil {
			a.log.Warn("failed checking teacher digest sent status for fallback", "teacher", teacherName, "error", err)
		}
		if sent {
			continue
		}

		if err := a.digestRepo.MarkTeacherDigestSent(ctx, teacherName, todayStr); err != nil {
			a.log.Warn("failed marking teacher digest as sent in fallback", "teacher", teacherName, "error", err)
		}

		a.log.Info("Sending 20:00 fallback digest for teacher", "teacher", teacherName)
		if _, sendErr := a.notifier.SendNextDayDigestForTeacher(ctx, a.teacherSvc, teacherName, users); sendErr != nil {
			a.log.Warn("failed sending 20:00 fallback digest for teacher", "teacher", teacherName, "error", sendErr)
		}
	}
}

func extractUniqueRecipients(users []entities.User) (map[groupSubgroupKey]struct{}, map[string]struct{}) {
	targets := make(map[groupSubgroupKey]struct{})
	teachers := make(map[string]struct{})
	for _, u := range users {
		if u.Group > 0 {
			targets[groupSubgroupKey{groupID: u.Group, subgroup: u.Subgroup}] = struct{}{}
		} else if u.TeacherName != "" {
			teachers[u.TeacherName] = struct{}{}
		}
	}
	return targets, teachers
}

func (a *App) notifyGroupsAfterLessons(
	ctx context.Context,
	localTime time.Time,
	dayIdx int,
	targets map[groupSubgroupKey]struct{},
	todayStr string,
	users []entities.User,
) {
	for target := range targets {
		sent, err := a.digestRepo.IsGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr)
		if err != nil {
			a.log.Warn("failed checking group digest sent status", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}
		if sent {
			continue
		}

		if !a.isGroupDayEnded(ctx, target.groupID, target.subgroup, dayIdx, localTime) {
			continue
		}

		if err := a.digestRepo.MarkGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr); err != nil {
			a.log.Warn("failed marking group digest as sent", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}

		a.log.Info("Lessons ended for group/subgroup, sending next day digest",
			"group", target.groupID,
			"subgroup", target.subgroup,
			"time", localTime.Format("15:04:05"),
		)

		if _, sendErr := a.notifier.SendNextDayDigestForGroupSubgroup(ctx, a.scheduleSvc, target.groupID, target.subgroup, users); sendErr != nil {
			a.log.Warn("failed sending next day digest for group/subgroup",
				"group", target.groupID,
				"subgroup", target.subgroup,
				"error", sendErr,
			)
		}
	}
}

func (a *App) notifyTeachersAfterLessons(
	ctx context.Context,
	localTime time.Time,
	dayIdx int,
	teachers map[string]struct{},
	todayStr string,
	users []entities.User,
) {
	for teacherName := range teachers {
		sent, err := a.digestRepo.IsTeacherDigestSent(ctx, teacherName, todayStr)
		if err != nil {
			a.log.Warn("failed checking teacher digest sent status", "teacher", teacherName, "error", err)
		}
		if sent {
			continue
		}

		if !a.isTeacherDayEnded(ctx, teacherName, dayIdx, localTime) {
			continue
		}

		if err := a.digestRepo.MarkTeacherDigestSent(ctx, teacherName, todayStr); err != nil {
			a.log.Warn("failed marking teacher digest as sent", "teacher", teacherName, "error", err)
		}

		a.log.Info("Lessons ended for teacher, sending next day digest",
			"teacher", teacherName,
			"time", localTime.Format("15:04:05"),
		)

		if _, sendErr := a.notifier.SendNextDayDigestForTeacher(ctx, a.teacherSvc, teacherName, users); sendErr != nil {
			a.log.Warn("failed sending next day digest for teacher",
				"teacher", teacherName,
				"error", sendErr,
			)
		}
	}
}
