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

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
)

// InlineQuery обрабатывает inline query запросы к боту (@mgkct_bot <запрос>)
func InlineQuery(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
	webAppURL string,
) tele.HandlerFunc {
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
			results := buildEmptyQueryResults(ctx, log, c, schedSvc, teacherSvc, userRepo, webAppURL)
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

			results := buildGroupQueryResults(ctx, log, schedSvc, groupID, subgroup, webAppURL)
			return c.Answer(&tele.QueryResponse{
				Results:   results,
				CacheTime: 60,
			})
		}

		// 3. Запрос преподавателя (поиск по фамилии / ФИО)
		results := buildTeacherQueryResults(ctx, log, teacherSvc, rawQuery, webAppURL)
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
	webAppURL string,
) tele.Results {
	if c.Query() == nil || c.Query().Sender == nil || userRepo == nil {
		return buildDefaultHelpResults()
	}

	userID := c.Query().Sender.ID

	// Проверяем, сохранен ли преподаватель
	if teacherName, err := userRepo.GetTeacher(ctx, userID); err == nil && teacherName != "" {
		res := buildTeacherQueryResults(ctx, log, teacherSvc, teacherName, webAppURL)
		if len(res) > 0 {
			return res
		}
	}

	// Проверяем, сохранена ли группа
	if groupID, subgroup, err := userRepo.GroupAndSubgroupByChatID(ctx, userID); err == nil && groupID > 0 {
		res := buildGroupQueryResults(ctx, log, schedSvc, groupID, subgroup, webAppURL)
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
		Description: "Введите фамилию (например: Кулецкая, Камлюк)",
		Text:        "Для просмотра расписания преподавателя укажите фамилию после имени бота, например:\n`@mgkct_bot Кулецкая`",
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
	webAppURL string,
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

	sort.Slice(allWeeks, func(i, j int) bool {
		return allWeeks[i].After(allWeeks[j])
	})

	const maxWeeks = 10
	if len(allWeeks) > maxWeeks {
		allWeeks = allWeeks[:maxWeeks]
	}

	curMonday := currentWeekMonday(time.Now())
	results := make(tele.Results, 0, len(allWeeks))

	for i, w := range allWeeks {
		grp, wErr := schedSvc.GetGroupScheduleByWeek(ctx, int32(groupID), w)
		if wErr != nil || grp == nil {
			continue
		}

		text := formatter.FormatScheduleWeekForSubgroup(grp, subgroup)
		desc := fmt.Sprintf("Группа %d", groupID)
		if subgroup > 0 {
			desc += fmt.Sprintf(" (%d п/г)", subgroup)
		}

		label := determineWeekLabel(w, curMonday)
		title := fmt.Sprintf("%s (%s)", label, formatWeekRange(w))

		article := &tele.ArticleResult{
			Title:       title,
			Description: desc,
			Text:        text,
		}
		article.SetResultID(hashResultID(fmt.Sprintf("grp:%d:%s:%d:%d", groupID, w.Format("2006-01-02"), subgroup, i)))
		article.SetParseMode(tele.ModeMarkdown)

		if markup := buildInlineMessageMarkup(webAppURL); markup != nil {
			article.SetReplyMarkup(markup)
		}

		results = append(results, article)
	}

	return results
}

func buildTeacherQueryResults(
	ctx context.Context,
	log *slog.Logger,
	teacherSvc TeacherService,
	query string,
	webAppURL string,
) tele.Results {
	matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
	if !ok {
		if len(candidates) > 0 {
			results := make(tele.Results, 0, len(candidates))
			for i, cand := range candidates {
				latest, err := teacherSvc.GetTeacherLatestSchedule(ctx, cand)
				if err != nil {
					continue
				}
				text := formatter.FormatTeacherScheduleWeek(latest)
				article := &tele.ArticleResult{
					Title:       cand,
					Description: fmt.Sprintf("Текущая неделя (%s)", formatWeekRange(latest.Week)),
					Text:        text,
				}
				article.SetResultID(hashResultID(fmt.Sprintf("cand:%s:%d", cand, i)))
				article.SetParseMode(tele.ModeMarkdown)
				if markup := buildInlineMessageMarkup(webAppURL); markup != nil {
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

	sort.Slice(allWeeks, func(i, j int) bool {
		return allWeeks[i].After(allWeeks[j])
	})

	const maxWeeks = 10
	if len(allWeeks) > maxWeeks {
		allWeeks = allWeeks[:maxWeeks]
	}

	curMonday := currentWeekMonday(time.Now())
	results := make(tele.Results, 0, len(allWeeks))

	for i, w := range allWeeks {
		tch, wErr := teacherSvc.GetTeacherScheduleByWeek(ctx, matched, w)
		if wErr != nil || tch == nil {
			continue
		}

		text := formatter.FormatTeacherScheduleWeek(tch)
		label := determineWeekLabel(w, curMonday)
		title := fmt.Sprintf("%s (%s)", label, formatWeekRange(w))

		article := &tele.ArticleResult{
			Title:       title,
			Description: matched,
			Text:        text,
		}
		article.SetResultID(hashResultID(fmt.Sprintf("tch:%s:%s:%d", matched, w.Format("2006-01-02"), i)))
		article.SetParseMode(tele.ModeMarkdown)

		if markup := buildInlineMessageMarkup(webAppURL); markup != nil {
			article.SetReplyMarkup(markup)
		}

		results = append(results, article)
	}

	return results
}

var minskLoc = time.FixedZone("MSK", 3*3600)

func currentWeekMonday(now time.Time) time.Time {
	localNow := now.In(minskLoc)
	weekday := localNow.Weekday()
	offset := (int(weekday) + 6) % 7 // Пн=0, Вт=1, ..., Вс=6
	return time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, minskLoc).AddDate(0, 0, -offset)
}

func determineWeekLabel(w time.Time, curMonday time.Time) string {
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

func buildInlineMessageMarkup(webAppURL string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var buttons []tele.Btn

	if webAppURL != "" {
		buttons = append(buttons, markup.WebApp("📱 Mini App", &tele.WebApp{URL: webAppURL}))
	}
	buttons = append(buttons, markup.QueryChat("🔍 Другое расписание", ""))

	if len(buttons) > 0 {
		markup.Inline(markup.Row(buttons...))
		return markup
	}

	return nil
}

func hashResultID(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:8])
}
