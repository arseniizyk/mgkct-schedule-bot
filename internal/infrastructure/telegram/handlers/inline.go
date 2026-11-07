package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

// InlineQuery обрабатывает inline query запросы к боту (@mgkct_bot <запрос>)
func InlineQuery(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
) tele.HandlerFunc {
	return InlineQueryWithClock(log, schedSvc, teacherSvc, userRepo, time.Now)
}

// InlineQueryWithClock позволяет инжектировать функцию времени для детерминированных тестов
func InlineQueryWithClock(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
	nowFunc func() time.Time,
) tele.HandlerFunc {
	if nowFunc == nil {
		nowFunc = time.Now
	}

	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		q := c.Query()
		if q == nil {
			return nil
		}

		rawQuery := strings.TrimSpace(q.Text)

		// 1. Пустой запрос: если пользователь привязал группу/преподавателя — показываем их, иначе подсказку
		if rawQuery == "" {
			results := buildEmptyQueryResults(ctx, log, c, schedSvc, teacherSvc, userRepo, nowFunc)
			return c.Answer(&tele.QueryResponse{
				Results:    results,
				CacheTime:  30,
				IsPersonal: true,
			})
		}

		// 2. Номер группы (например, "88" или "88 1")
		fields := strings.Fields(rawQuery)
		if groupID, err := strconv.Atoi(fields[0]); err == nil && groupID > 0 {
			subgroup := 0
			if len(fields) > 1 {
				if sg, err := strconv.Atoi(fields[1]); err == nil && (sg == 1 || sg == 2) {
					subgroup = sg
				}
			}

			results := buildGroupQueryResults(ctx, log, schedSvc, groupID, subgroup, nowFunc)
			return c.Answer(&tele.QueryResponse{
				Results:   results,
				CacheTime: 60,
			})
		}

		// 3. Запрос преподавателя (поиск по фамилии / ФИО)
		results := buildTeacherQueryResults(ctx, log, teacherSvc, rawQuery, nowFunc)
		return c.Answer(&tele.QueryResponse{
			Results:   results,
			CacheTime: 60,
		})
	}
}

func buildEmptyQueryResults(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
	nowFunc func() time.Time,
) tele.Results {
	if c.Query() == nil || c.Query().Sender == nil || userRepo == nil {
		return buildDefaultHelpResults()
	}

	userID := c.Query().Sender.ID
	groupID, subgroup, teacherName, err := userRepo.GetUserProfile(ctx, userID)
	if err != nil {
		return buildDefaultHelpResults()
	}

	if teacherName != "" {
		res := buildTeacherQueryResults(ctx, log, teacherSvc, teacherName, nowFunc)
		if len(res) > 0 {
			return res
		}
	}

	if groupID > 0 {
		res := buildGroupQueryResults(ctx, log, schedSvc, groupID, subgroup, nowFunc)
		if len(res) > 0 {
			return res
		}
	}

	return buildDefaultHelpResults()
}

func buildDefaultHelpResults() tele.Results {
	r1 := &tele.ArticleResult{
		Title:       "🔍 Расписание группы",
		Description: "Введите номер группы (например: 80, 88)",
		Text:        "Для просмотра расписания группы укажите её номер после имени бота, например:\n`@mgkct_bot 88`",
	}
	r1.SetResultID("help_group")
	r1.SetParseMode(tele.ModeMarkdown)

	r2 := &tele.ArticleResult{
		Title:       "🔍 Расписание преподавателя",
		Description: "Введите фамилию (например: Егорова, Павлов)",
		Text:        "Для просмотра расписания преподавателя укажите фамилию после имени бота, например:\n`@mgkct_bot Егорова`",
	}
	r2.SetResultID("help_teacher")
	r2.SetParseMode(tele.ModeMarkdown)

	return tele.Results{r1, r2}
}

func buildGroupQueryResults(
	ctx context.Context,
	log *slog.Logger,
	schedSvc ScheduleService,
	groupID int,
	subgroup int,
	nowFunc func() time.Time,
) tele.Results {
	hasGroup, err := schedSvc.HasGroup(ctx, int32(groupID))
	if err != nil || !hasGroup {
		return tele.Results{
			buildNotFoundArticle(
				fmt.Sprintf("not_found_grp_%d", groupID),
				fmt.Sprintf("Группа %d не найдена", groupID),
				"Проверьте номер группы и повторите попытку",
			),
		}
	}

	allWeeks, err := schedSvc.GetAllWeeks(ctx)
	if err != nil || len(allWeeks) == 0 {
		latestGroup, lErr := schedSvc.GetGroupLatestSchedule(ctx, int32(groupID))
		if lErr != nil {
			log.Warn("inline: failed getting group schedule", "group_id", groupID, "error", lErr)
			return nil
		}
		allWeeks = []time.Time{latestGroup.Week}
	}

	curMonday := timeutil.CurrentWeekMonday(nowFunc())
	sortWeeksByRelevance(allWeeks, curMonday)

	const maxWeeks = 4
	if len(allWeeks) > maxWeeks {
		allWeeks = allWeeks[:maxWeeks]
	}

	results := make(tele.Results, 0, len(allWeeks)*6)

	for wIdx, w := range allWeeks {
		grp, wErr := schedSvc.GetGroupScheduleByWeek(ctx, int32(groupID), w)
		if wErr != nil || grp == nil || len(grp.Days) == 0 {
			continue
		}

		filtered := formatter.FilterGroupForSubgroup(grp, subgroup)
		weekLabel := determineWeekLabel(w, curMonday)
		weeks, wNavErr := schedSvc.GetAvailableWeeks(ctx, grp.Week)
		if wNavErr != nil {
			weeks = entities.WeekNavigation{}
		}
		if weeks.Current.IsZero() {
			weeks.Current = grp.Week
		}

		for dayIdx, day := range filtered.Days {
			if day == nil {
				continue
			}

			dayTitle := day.Name
			if dayTitle == "" && dayIdx < len(defaultDayNames) {
				dayTitle = defaultDayNames[dayIdx]
			}

			tag := formatter.DayRelativeLabelWithNow(grp.Week, dayIdx, nowFunc())
			title := fmt.Sprintf("%s%s", dayTitle, tag)

			desc := fmt.Sprintf("Группа %d • %s", groupID, weekLabel)
			if subgroup > 0 {
				desc = fmt.Sprintf("Группа %d (%d п/г) • %s", groupID, subgroup, weekLabel)
			}

			text, err := formatter.FormatScheduleDayAtForSubgroup(grp, dayIdx, subgroup)
			if err != nil {
				continue
			}

			article := &tele.ArticleResult{
				Title:       title,
				Description: desc,
				Text:        text,
			}
			article.SetResultID(hashResultID(fmt.Sprintf("grp:%d:%s:%d:%d:%d", groupID, w.Format("2006-01-02"), subgroup, wIdx, dayIdx)))
			article.SetParseMode(tele.ModeMarkdown)

			if markup := keyboard.InlineDayKeyboard(groupID, dayIdx, len(grp.Days), subgroup, weeks); markup != nil {
				article.SetReplyMarkup(markup)
			}

			results = append(results, article)
		}
	}

	return results
}

func buildTeacherQueryResults(
	ctx context.Context,
	log *slog.Logger,
	teacherSvc TeacherService,
	query string,
	nowFunc func() time.Time,
) tele.Results {
	matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
	if !ok {
		if len(candidates) > 0 {
			const maxCandidates = 10
			if len(candidates) > maxCandidates {
				candidates = candidates[:maxCandidates]
			}

			results := make(tele.Results, 0, len(candidates))
			for i, cand := range candidates {
				latest, err := teacherSvc.GetTeacherLatestSchedule(ctx, cand)
				if err != nil {
					continue
				}
				dayIdx := formatter.EffectiveTeacherDayIndex(latest)
				text, err := formatter.FormatTeacherScheduleDayAt(latest, dayIdx)
				if err != nil {
					text = formatter.FormatTeacherScheduleDay(latest)
				}
				weeks, wNavErr := teacherSvc.GetAvailableWeeks(ctx, cand, latest.Week)
				if wNavErr != nil {
					weeks = entities.WeekNavigation{}
				}
				if weeks.Current.IsZero() {
					weeks.Current = latest.Week
				}
				article := &tele.ArticleResult{
					Title:       cand,
					Description: fmt.Sprintf("Текущая неделя (%s)", formatWeekRange(latest.Week)),
					Text:        text,
				}
				article.SetResultID(hashResultID(fmt.Sprintf("cand:%s:%d", cand, i)))
				article.SetParseMode(tele.ModeMarkdown)
				if markup := keyboard.InlineTeacherDayKeyboard(cand, dayIdx, len(latest.Days), weeks); markup != nil {
					article.SetReplyMarkup(markup)
				}
				results = append(results, article)
			}
			return results
		}

		return tele.Results{
			buildNotFoundArticle(
				"not_found_teacher",
				"Преподаватель не найден",
				"Проверьте написание фамилии и повторите попытку",
			),
		}
	}

	allWeeks, err := teacherSvc.GetAllWeeks(ctx, matched)
	if err != nil || len(allWeeks) == 0 {
		latestTeacher, lErr := teacherSvc.GetTeacherLatestSchedule(ctx, matched)
		if lErr != nil {
			log.Warn("inline: failed getting teacher schedule", "name", matched, "error", lErr)
			return nil
		}
		allWeeks = []time.Time{latestTeacher.Week}
	}

	curMonday := timeutil.CurrentWeekMonday(nowFunc())
	sortWeeksByRelevance(allWeeks, curMonday)

	const maxWeeks = 4
	if len(allWeeks) > maxWeeks {
		allWeeks = allWeeks[:maxWeeks]
	}

	results := make(tele.Results, 0, len(allWeeks)*6)

	for wIdx, w := range allWeeks {
		tch, wErr := teacherSvc.GetTeacherScheduleByWeek(ctx, matched, w)
		if wErr != nil || tch == nil || len(tch.Days) == 0 {
			continue
		}

		weekLabel := determineWeekLabel(w, curMonday)
		weeks, wNavErr := teacherSvc.GetAvailableWeeks(ctx, matched, tch.Week)
		if wNavErr != nil {
			weeks = entities.WeekNavigation{}
		}
		if weeks.Current.IsZero() {
			weeks.Current = tch.Week
		}

		for dayIdx, day := range tch.Days {
			if day == nil {
				continue
			}

			dayTitle := day.Name
			if dayTitle == "" && dayIdx < len(defaultDayNames) {
				dayTitle = defaultDayNames[dayIdx]
			}

			tag := formatter.DayRelativeLabelWithNow(tch.Week, dayIdx, nowFunc())
			title := fmt.Sprintf("%s%s", dayTitle, tag)
			desc := fmt.Sprintf("%s • %s", matched, weekLabel)

			text, err := formatter.FormatTeacherScheduleDayAt(tch, dayIdx)
			if err != nil {
				continue
			}

			article := &tele.ArticleResult{
				Title:       title,
				Description: desc,
				Text:        text,
			}
			article.SetResultID(hashResultID(fmt.Sprintf("tch:%s:%s:%d:%d", matched, w.Format("2006-01-02"), wIdx, dayIdx)))
			article.SetParseMode(tele.ModeMarkdown)

			if markup := keyboard.InlineTeacherDayKeyboard(matched, dayIdx, len(tch.Days), weeks); markup != nil {
				article.SetReplyMarkup(markup)
			}

			results = append(results, article)
		}
	}

	return results
}

var defaultDayNames = []string{
	"Понедельник",
	"Вторник",
	"Среда",
	"Четверг",
	"Пятница",
	"Суббота",
}

func sortWeeksByRelevance(weeks []time.Time, curMonday time.Time) {
	sort.Slice(weeks, func(i, j int) bool {
		wi := weekRelevanceWeight(weeks[i], curMonday)
		wj := weekRelevanceWeight(weeks[j], curMonday)
		if wi != wj {
			return wi < wj
		}
		return weeks[i].Before(weeks[j])
	})
}

func weekRelevanceWeight(w, curMonday time.Time) int {
	wNorm := time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, minskLoc)
	curNorm := time.Date(curMonday.Year(), curMonday.Month(), curMonday.Day(), 0, 0, 0, 0, minskLoc)
	diffDays := int(math.Round(wNorm.Sub(curNorm).Hours() / 24))
	switch {
	case diffDays == 0:
		return 0
	case diffDays == 7:
		return 1
	case diffDays > 7:
		return 2
	default:
		return 3
	}
}

var minskLoc = timeutil.MinskLocation

func determineWeekLabel(w, curMonday time.Time) string {
	wNorm := time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, minskLoc)
	curNorm := time.Date(curMonday.Year(), curMonday.Month(), curMonday.Day(), 0, 0, 0, 0, minskLoc)
	diffDays := int(math.Round(wNorm.Sub(curNorm).Hours() / 24))
	switch {
	case diffDays == 0:
		return "Текущая неделя"
	case diffDays == 7:
		return "Следующая неделя"
	case diffDays > 7:
		return "Будущая неделя"
	case diffDays == -7:
		return "Предыдущая неделя"
	default:
		return "Неделя"
	}
}

func formatWeekRange(monday time.Time) string {
	saturday := monday.AddDate(0, 0, 5)
	return fmt.Sprintf("%s – %s", monday.Format("02.01"), saturday.Format("02.01"))
}

func buildNotFoundArticle(id, title, desc string) *tele.ArticleResult {
	article := &tele.ArticleResult{
		Title:       title,
		Description: desc,
		Text:        fmt.Sprintf("*%s*\n%s", title, desc),
	}
	article.SetResultID(hashResultID(id))
	article.SetParseMode(tele.ModeMarkdown)
	return article
}

func hashResultID(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:8])
}
