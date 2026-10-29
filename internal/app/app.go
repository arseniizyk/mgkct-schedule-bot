package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/db"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/dictionary"
	httpserver "github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/notifier"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type App struct {
	log         *slog.Logger
	cfg         *config.Config
	database    *sql.DB
	bot         *tele.Bot
	scraper     *service.ScraperService
	notifier    *notifier.Notifier
	eventCh     chan service.Event
	httpSrv     *httpserver.Server
	scheduleSvc *service.ScheduleService
	teacherSvc  *service.TeacherService
	digestRepo  *repository.DigestSentRepository
}

func New(log *slog.Logger, cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Подключение к SQLite с автоматическими миграциями
	database, err := db.Connect(ctx, &cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("app: database connect: %w", err)
	}

	// 2. Репозитории
	scheduleRepo := repository.NewScheduleRepository(database)
	teacherRepo := repository.NewTeacherScheduleRepository(database)
	facultyRepo := repository.NewFacultyRepository(database)
	userRepo := repository.NewUserRepository(database)
	digestRepo := repository.NewDigestSentRepository(database)

	// 3. Сервисы и справочники
	subjectDict := dictionary.NewSubjectDictionary()
	scheduleSvc := service.NewScheduleService(log, scheduleRepo)
	teacherSvc := service.NewTeacherService(log, teacherRepo, facultyRepo, subjectDict)
	analyticsSvc := service.NewAnalyticsService(log, scheduleRepo, teacherRepo, subjectDict)

	studentParser := parser.New(log)
	teacherParser := parser.NewTeacherParser(log)
	facultyParser := parser.NewFacultyParser(log)

	scraperSvc := service.NewScraperService(
		log,
		cfg.Scraper,
		studentParser,
		teacherParser,
		scheduleRepo,
		teacherRepo,
		scheduleSvc,
	)
	scraperSvc.SetFaculty(facultyParser, facultyRepo)

	// 4. Telegram Бот
	botPref := tele.Settings{
		URL:    cfg.Bot.APIURL,
		Token:  cfg.Bot.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	bot, err := tele.NewBot(botPref)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("app: create bot: %w", err)
	}

	stateMgr := state.NewMemoryManager()

	// Сброс интерактивного состояния при вводе любой команды с "/"
	bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Message() != nil && strings.HasPrefix(c.Message().Text, "/") {
				_ = stateMgr.Clear(c.Chat().ID)
			}
			return next(c)
		}
	})

	// Инициализация клавиатуры с WebApp URL (если настроен)
	kbd.InitKeyboard(cfg.Bot.WebAppURL)

	registerBotHandlers(
		bot,
		log,
		cfg,
		stateMgr,
		userRepo,
		scheduleSvc,
		teacherSvc,
		analyticsSvc,
		subjectDict,
	)

	eventCh := make(chan service.Event, 100)
	notif := notifier.New(log, bot, userRepo, cfg.Bot.AdminIDs)

	httpSrv := httpserver.New(
		log,
		cfg,
		database,
		scheduleSvc,
		teacherSvc,
		analyticsSvc,
		userRepo,
	)

	return &App{
		log:         log,
		cfg:         cfg,
		database:    database,
		bot:         bot,
		scraper:     scraperSvc,
		notifier:    notif,
		eventCh:     eventCh,
		httpSrv:     httpSrv,
		scheduleSvc: scheduleSvc,
		teacherSvc:  teacherSvc,
		digestRepo:  digestRepo,
	}, nil
}

func (a *App) Run() error {
	log := a.log.With("operation", "app.App.Run")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers.SetBroadcastContext(ctx)

	// 1. Запуск обработчика событий рассылки
	go a.listenEvents(ctx)

	// 2. Защита от повторной рассылки при перезапуске:
	//    если бот стартует после окончания пар — помечаем всех как уже уведомлённых.
	a.seedDigestLogIfNeeded(ctx)

	// 3. Запуск планировщика вечернего дайджеста
	go a.startEveningDigestScheduler(ctx)

	// 3. Запуск фонового скрапера
	go a.scraper.Start(ctx, a.eventCh)

	// 4. Запуск HTTP сервера (Healthcheck, REST API, WebApp)
	go func() {
		if err := a.httpSrv.Start(); err != nil {
			log.Error("http server failed", "error", err)
		}
	}()

	// 5. Ожидание сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Shutting down gracefully...")
		a.bot.Stop()
		cancel()

		if a.httpSrv != nil {
			shutdownCtx, sCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer sCancel()
			_ = a.httpSrv.Shutdown(shutdownCtx)
		}

		_ = a.database.Close()
	}()

	log.Info("Bot started!", "username", a.bot.Me.Username)
	a.bot.Start()

	return nil
}

func (a *App) listenEvents(ctx context.Context) {
	log := a.log.With("operation", "app.App.listenEvents")

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-a.eventCh:
			switch event.Type {
			case service.EventNewWeek:
				log.Info("Received EventNewWeek, broadcasting to users", "week", event.Week)
				if err := a.notifier.HandleNewWeek(ctx, event.Week); err != nil {
					log.Error("failed broadcasting new week", "error", err)
				}
			case service.EventGroupScheduleUpdated:
				log.Info("Received EventGroupScheduleUpdated", "group_id", event.Group.ID)
				if err := a.notifier.HandleGroupScheduleUpdate(ctx, event.Group, event.Diff); err != nil {
					log.Error("failed broadcasting group update", "group_id", event.Group.ID, "error", err)
				}
			case service.EventTeacherScheduleUpdated:
				log.Info("Received EventTeacherScheduleUpdated", "name", event.Teacher.Name)
				if err := a.notifier.HandleTeacherScheduleUpdate(ctx, event.Teacher, event.Diff); err != nil {
					log.Error("failed broadcasting teacher update", "name", event.Teacher.Name, "error", err)
				}
			case service.EventScraperError:
				log.Warn("Received EventScraperError", "consecutive_fails", event.ConsecutiveFails, "error", event.Err)
				if err := a.notifier.HandleScraperError(ctx, event.Err, event.ConsecutiveFails); err != nil {
					log.Error("failed sending scraper error alert", "error", err)
				}
			case service.EventScraperRecovered:
				log.Info("Received EventScraperRecovered")
				if err := a.notifier.HandleScraperRecovered(ctx); err != nil {
					log.Error("failed sending scraper recovery alert", "error", err)
				}
			}
		}
	}
}

type groupSubgroupKey struct {
	groupID  int
	subgroup int
}

// seedDigestLogIfNeeded вызывается при старте приложения.
// Если сейчас рабочий день и пары уже закончились — помечает все группы и преподавателей
// как уже уведомлённых на сегодня, не отправляя сообщений.
// Это предотвращает повторную рассылку при перезапуске или деплое новой версии.
func (a *App) seedDigestLogIfNeeded(ctx context.Context) {
	const op = "app.App.seedDigestLogIfNeeded"
	log := a.log.With("operation", op)

	minskLoc := time.FixedZone("MSK", 3*3600)
	now := time.Now().In(minskLoc)
	weekday := now.Weekday()

	// В воскресенье до 20:00 рассылка не происходит — сидировать нечего.
	if weekday == time.Sunday && now.Hour() < 20 {
		return
	}

	todayStr := now.Format("2006-01-02")

	users, err := a.notifier.UsersForNotification(ctx)
	if err != nil || len(users) == 0 {
		return
	}

	uniqueTargets, uniqueTeachers := extractUniqueRecipients(users)
	seeded := 0

	// Если уже 20:00 или позже (в любой день) — вечерняя рассылка
	// уже должна была состояться, поэтому сидируем всех получателей.
	if now.Hour() >= 20 {
		for target := range uniqueTargets {
			if err := a.digestRepo.MarkGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr); err != nil {
				log.Warn("failed seeding group digest log", "group", target.groupID, "subgroup", target.subgroup, "error", err)
				continue
			}
			seeded++
		}
		for teacherName := range uniqueTeachers {
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

	// Для учебных дней (Пн..Сб) до 20:00 проверяем окончание пар:
	dayIdx := int(weekday - 1) // 0..5 для Пн..Сб

	// Группы
	for target := range uniqueTargets {
		grp, err := a.scheduleSvc.GetGroupLatestSchedule(ctx, int32(target.groupID))
		if err != nil || dayIdx >= len(grp.Days) {
			continue
		}

		filtered := formatter.FilterGroupForSubgroup(grp, target.subgroup)
		if dayIdx >= len(filtered.Days) || filtered.Days[dayIdx] == nil {
			continue
		}

		lastSubject := formatter.FindLastSubject(filtered.Days[dayIdx].Subjects)
		if lastSubject == -1 {
			continue
		}

		endTime, ok := formatter.GetDayEndTime(dayIdx, lastSubject+1, minskLoc)
		if !ok || now.Before(endTime) {
			// Пары ещё не закончились — ничего не трогаем.
			continue
		}

		// Пары уже закончились: засеиваем запись, не отправляя ничего.
		if err := a.digestRepo.MarkGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr); err != nil {
			log.Warn("failed seeding group digest log", "group", target.groupID, "subgroup", target.subgroup, "error", err)
			continue
		}
		seeded++
	}

	// Преподаватели
	for teacherName := range uniqueTeachers {
		teacher, err := a.teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
		if err != nil || dayIdx >= len(teacher.Days) {
			continue
		}

		lastSubject := formatter.FindLastSubject(teacher.Days[dayIdx].Subjects)
		if lastSubject == -1 {
			continue
		}

		endTime, ok := formatter.GetDayEndTime(dayIdx, lastSubject+1, minskLoc)
		if !ok || now.Before(endTime) {
			continue
		}

		if err := a.digestRepo.MarkTeacherDigestSent(ctx, teacherName, todayStr); err != nil {
			log.Warn("failed seeding teacher digest log", "teacher", teacherName, "error", err)
			continue
		}
		seeded++
	}

	if seeded > 0 {
		log.Info("seeded digest log on startup to prevent re-broadcast", "seeded_count", seeded, "date", todayStr)
	}
}

func (a *App) startEveningDigestScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	minskLoc := time.FixedZone("MSK", 3*3600)
	var lastEveningDate string

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			localTime := t.In(minskLoc)
			todayStr := localTime.Format("2006-01-02")
			todayWeekday := localTime.Weekday() // 0=Вс, 1=Пн, ... 6=Сб

			// Очищаем устаревшие записи дайджеста (записи старше вчера)
			yesterday := localTime.AddDate(0, 0, -1).Format("2006-01-02")
			if err := a.digestRepo.DeleteOldEntries(ctx, yesterday); err != nil {
				a.log.Warn("failed cleaning up old digest log entries", "error", err)
			}

			// 1. В учебные дни (Пн..Сб) до 20:00: проверяем подписчиков: если их пары на сегодня уже закончились — высылаем расписание на следующий рабочий день
			if todayWeekday >= time.Monday && todayWeekday <= time.Saturday && localTime.Hour() < 20 {
				dayIdx := int(todayWeekday - 1) // 0..5 для Пн..Сб

				users, err := a.notifier.UsersForNotification(ctx)
				if err == nil && len(users) > 0 {
					uniqueTargets, uniqueTeachers := extractUniqueRecipients(users)
					a.notifyGroupsAfterLessons(ctx, localTime, dayIdx, minskLoc, uniqueTargets, todayStr)
					a.notifyTeachersAfterLessons(ctx, localTime, dayIdx, minskLoc, uniqueTeachers, todayStr)
				}
			}

			// 2. В 20:00 (в воскресенье — для всех, в будни/субботу — для групп и преподавателей без пар или кто ещё не получил дайджест)
			if localTime.Hour() >= 20 && lastEveningDate != todayStr {
				lastEveningDate = todayStr
				a.log.Info("Triggering scheduled evening digest fallback at 20:00", "time", localTime.Format("15:04:05"))
				a.sendEveningDigestFallback(ctx, todayStr)
			}
		}
	}
}

func (a *App) sendEveningDigestFallback(ctx context.Context, todayStr string) {
	log := a.log.With("operation", "app.App.sendEveningDigestFallback")
	users, err := a.notifier.UsersForNotification(ctx)
	if err != nil || len(users) == 0 {
		return
	}

	uniqueTargets, uniqueTeachers := extractUniqueRecipients(users)

	// Отправка группам/подгруппам, не получившим дайджест ранее
	for target := range uniqueTargets {
		sent, err := a.digestRepo.IsGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr)
		if err != nil {
			log.Warn("failed checking group digest sent status", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}
		if sent {
			continue
		}

		if err := a.digestRepo.MarkGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr); err != nil {
			log.Warn("failed marking group digest as sent", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}

		if _, sendErr := a.notifier.SendNextDayDigestForGroupSubgroup(ctx, a.scheduleSvc, target.groupID, target.subgroup); sendErr != nil {
			log.Warn("failed sending evening digest for group/subgroup",
				"group", target.groupID,
				"subgroup", target.subgroup,
				"error", sendErr,
			)
		}
	}

	// Отправка преподавателям, не получившим дайджест ранее
	for teacherName := range uniqueTeachers {
		sent, err := a.digestRepo.IsTeacherDigestSent(ctx, teacherName, todayStr)
		if err != nil {
			log.Warn("failed checking teacher digest sent status", "teacher", teacherName, "error", err)
		}
		if sent {
			continue
		}

		if err := a.digestRepo.MarkTeacherDigestSent(ctx, teacherName, todayStr); err != nil {
			log.Warn("failed marking teacher digest as sent", "teacher", teacherName, "error", err)
		}

		if _, sendErr := a.notifier.SendNextDayDigestForTeacher(ctx, a.teacherSvc, teacherName); sendErr != nil {
			log.Warn("failed sending evening digest for teacher",
				"teacher", teacherName,
				"error", sendErr,
			)
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
	loc *time.Location,
	targets map[groupSubgroupKey]struct{},
	todayStr string,
) {
	for target := range targets {
		// Проверяем в БД: уже отправляли дайджест сегодня для этой группы/подгруппы?
		sent, err := a.digestRepo.IsGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr)
		if err != nil {
			a.log.Warn("failed checking group digest sent status", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}
		if sent {
			continue
		}

		grp, err := a.scheduleSvc.GetGroupLatestSchedule(ctx, int32(target.groupID))
		if err != nil || dayIdx >= len(grp.Days) {
			continue
		}

		// Фильтруем расписание сегодняшнего дня для конкретной подгруппы
		filtered := formatter.FilterGroupForSubgroup(grp, target.subgroup)
		if dayIdx >= len(filtered.Days) || filtered.Days[dayIdx] == nil {
			continue
		}

		lastSubject := formatter.FindLastSubject(filtered.Days[dayIdx].Subjects)
		if lastSubject == -1 {
			continue
		}

		endTime, ok := formatter.GetDayEndTime(dayIdx, lastSubject+1, loc)
		if !ok || localTime.Before(endTime) {
			continue
		}

		// Помечаем как отправленное в БД до фактической отправки,
		// чтобы избежать повторной рассылки даже при ошибке отправки.
		if err := a.digestRepo.MarkGroupDigestSent(ctx, target.groupID, target.subgroup, todayStr); err != nil {
			a.log.Warn("failed marking group digest as sent", "group", target.groupID, "subgroup", target.subgroup, "error", err)
		}

		a.log.Info("Lessons ended for group/subgroup, sending next day digest",
			"group", target.groupID,
			"subgroup", target.subgroup,
			"time", localTime.Format("15:04:05"),
		)

		if _, sendErr := a.notifier.SendNextDayDigestForGroupSubgroup(ctx, a.scheduleSvc, target.groupID, target.subgroup); sendErr != nil {
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
	loc *time.Location,
	teachers map[string]struct{},
	todayStr string,
) {
	for teacherName := range teachers {
		// Проверяем в БД: уже отправляли дайджест сегодня для этого преподавателя?
		sent, err := a.digestRepo.IsTeacherDigestSent(ctx, teacherName, todayStr)
		if err != nil {
			a.log.Warn("failed checking teacher digest sent status", "teacher", teacherName, "error", err)
		}
		if sent {
			continue
		}

		teacher, err := a.teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
		if err != nil || dayIdx >= len(teacher.Days) {
			continue
		}

		lastSubject := formatter.FindLastSubject(teacher.Days[dayIdx].Subjects)
		if lastSubject == -1 {
			continue
		}

		endTime, ok := formatter.GetDayEndTime(dayIdx, lastSubject+1, loc)
		if !ok || localTime.Before(endTime) {
			continue
		}

		// Помечаем как отправленное в БД до фактической отправки,
		// чтобы избежать повторной рассылки даже при ошибке отправки.
		if err := a.digestRepo.MarkTeacherDigestSent(ctx, teacherName, todayStr); err != nil {
			a.log.Warn("failed marking teacher digest as sent", "teacher", teacherName, "error", err)
		}

		a.log.Info("Lessons ended for teacher, sending next day digest",
			"teacher", teacherName,
			"time", localTime.Format("15:04:05"),
		)

		if _, sendErr := a.notifier.SendNextDayDigestForTeacher(ctx, a.teacherSvc, teacherName); sendErr != nil {
			a.log.Warn("failed sending next day digest for teacher", "teacher", teacherName, "error", sendErr)
		}
	}
}

func registerBotHandlers(
	bot *tele.Bot,
	log *slog.Logger,
	cfg *config.Config,
	stateMgr *state.MemoryManager,
	userRepo *repository.UserRepository,
	scheduleSvc *service.ScheduleService,
	teacherSvc *service.TeacherService,
	analyticsSvc *service.AnalyticsService,
	subjectDict *dictionary.SubjectDictionary,
) {
	// Регистрация команд
	bot.Handle("/start", handlers.Start(log, userRepo))
	bot.Handle("/help", handlers.Help(cfg.Bot.AdminIDs))
	bot.Handle("/app", handlers.App(cfg.Bot.WebAppURL))
	bot.Handle("/calls", handlers.Calls())
	bot.Handle("/cancel", handlers.Cancel(log, stateMgr))
	bot.Handle("/setgroup", handlers.SetGroup(log, userRepo, stateMgr))
	bot.Handle("/group", handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/day", handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/week", handlers.Week(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/teachers", handlers.Teachers(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/setteacher", handlers.SetTeacher(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/teacher", handlers.TeacherDay(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/teacherweek", handlers.Teachers(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/subject", handlers.Subject(subjectDict, stateMgr))
	bot.Handle("/cabinet", handlers.Cabinet(log, analyticsSvc, stateMgr))
	bot.Handle("/auditory", handlers.Cabinet(log, analyticsSvc, stateMgr))
	bot.Handle("/endings", handlers.Endings(log, analyticsSvc))
	bot.Handle("/stats", handlers.Stats(log, analyticsSvc, userRepo, teacherSvc))
	bot.Handle("/archive", handlers.Archive(log, scheduleSvc, teacherSvc, userRepo, stateMgr))
	bot.Handle("/notify", handlers.Notify(log, userRepo))

	// Команды администратора
	adminMiddleware := handlers.AdminOnly(cfg.Bot.AdminIDs)
	bot.Handle("/userstats", handlers.UserStats(log, userRepo), adminMiddleware)
	bot.Handle("/broadcast", handlers.Broadcast(log, stateMgr), adminMiddleware)
	bot.Handle("/botinfo", handlers.BotInfo(log, userRepo), adminMiddleware)

	// Кнопки Reply-клавиатуры
	bot.Handle(kbd.BtnDay, handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle(kbd.BtnWeek, handlers.Week(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle(kbd.BtnCalls, handlers.Calls())
	bot.Handle(kbd.BtnTeachers, handlers.Teachers(log, teacherSvc, userRepo, stateMgr))

	// Инлайн-коллбэки и ввод текста
	bot.Handle(tele.OnCallback, handlers.CallbacksHandler(log, scheduleSvc, teacherSvc, analyticsSvc, userRepo, stateMgr, bot, cfg.Bot.AdminIDs))
	bot.Handle(tele.OnText, handlers.StatesHandler(log, userRepo, stateMgr, teacherSvc, scheduleSvc, analyticsSvc, subjectDict, bot, cfg.Bot.AdminIDs))
}
