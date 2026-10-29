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

	// Регистрация команд
	bot.Handle("/start", handlers.Start(log, userRepo))
	bot.Handle("/app", handlers.App(cfg.Bot.WebAppURL))
	bot.Handle("/calls", handlers.Calls())
	bot.Handle("/cancel", handlers.Cancel(log, stateMgr))
	bot.Handle("/setgroup", handlers.SetGroup(log, userRepo, stateMgr))
	bot.Handle("/group", handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/day", handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/week", handlers.Week(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/teachers", handlers.Teachers(log, teacherSvc, stateMgr))
	bot.Handle("/setteacher", handlers.SetTeacher(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/teacher", handlers.TeacherDay(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/teacherweek", handlers.TeacherWeek(log, teacherSvc, userRepo))
	bot.Handle("/subject", handlers.Subject(subjectDict, stateMgr))
	bot.Handle("/cabinet", handlers.Cabinet(log, analyticsSvc, stateMgr))
	bot.Handle("/auditory", handlers.Cabinet(log, analyticsSvc, stateMgr))
	bot.Handle("/cabinets", handlers.Cabinets(log, analyticsSvc))
	bot.Handle("/endings", handlers.Endings(log, analyticsSvc))
	bot.Handle("/stats", handlers.Stats(log, analyticsSvc, userRepo, teacherSvc))
	bot.Handle("/archive", handlers.Archive(log, scheduleSvc, teacherSvc, userRepo, stateMgr))
	bot.Handle("/notify", handlers.Notify(log, userRepo))

	// Кнопки Reply-клавиатуры
	bot.Handle(kbd.BtnDay, handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle(kbd.BtnWeek, handlers.Week(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle(kbd.BtnCalls, handlers.Calls())
	bot.Handle(kbd.BtnTeachers, handlers.Teachers(log, teacherSvc, stateMgr))

	// Инлайн-коллбэки и ввод текста
	bot.Handle(tele.OnCallback, handlers.CallbacksHandler(log, scheduleSvc, teacherSvc, analyticsSvc, userRepo, stateMgr))
	bot.Handle(tele.OnText, handlers.StatesHandler(log, userRepo, stateMgr, teacherSvc, scheduleSvc, analyticsSvc, subjectDict))

	eventCh := make(chan service.Event, 100)
	notif := notifier.New(log, bot, userRepo, cfg.Bot.AdminChatID)

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
	}, nil
}

func (a *App) Run() error {
	log := a.log.With("operation", "app.App.Run")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Запуск обработчика событий рассылки
	go a.listenEvents(ctx)

	// 2. Запуск планировщика вечернего дайджеста
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

func (a *App) startEveningDigestScheduler(ctx context.Context) {
	log := a.log.With("operation", "app.App.startEveningDigestScheduler")
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	minskLoc := time.FixedZone("MSK", 3*3600)
	sentGroupsToday := make(map[int]bool)
	sentTeachersToday := make(map[string]bool)
	sentUsersToday := make(map[int64]bool)
	var currentDate string
	var lastEveningDate string

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			localTime := t.In(minskLoc)
			todayStr := localTime.Format("2006-01-02")
			todayWeekday := localTime.Weekday() // 0=Вс, 1=Пн, ... 6=Сб

			// Сброс кэша отправленных уведомлений при наступлении новых суток
			if currentDate != todayStr {
				currentDate = todayStr
				clear(sentGroupsToday)
				clear(sentTeachersToday)
				clear(sentUsersToday)
			}

			// 1. Проверяем подписчиков: если их пары на сегодня уже закончились — высылаем расписание на завтра
			if todayWeekday >= time.Monday && todayWeekday <= time.Saturday && localTime.Hour() < 20 {
				dayIdx := int(todayWeekday - 1) // 0..5 для Пн..Сб

				users, err := a.notifier.UsersForNotification(ctx)
				if err == nil && len(users) > 0 {
					uniqueGroups, uniqueTeachers := extractUniqueRecipients(users)
					a.notifyGroupsAfterLessons(ctx, localTime, dayIdx, minskLoc, uniqueGroups, sentGroupsToday, sentUsersToday)
					a.notifyTeachersAfterLessons(ctx, localTime, dayIdx, minskLoc, uniqueTeachers, sentTeachersToday, sentUsersToday)
				}
			}

			// 2. В 20:00 (или в воскресенье/выходные) отправляем общий вечерний дайджест тем, у кого сегодня не было пар или кто еще не получил
			if localTime.Hour() >= 20 && lastEveningDate != todayStr {
				lastEveningDate = todayStr
				log.Info("Triggering scheduled evening digest fallback", "time", localTime.Format("15:04:05"))
				if err := a.notifier.SendEveningDigest(ctx, a.scheduleSvc, a.teacherSvc, sentUsersToday); err != nil {
					log.Error("error during evening digest broadcast", "error", err)
				}
			}
		}
	}
}

func extractUniqueRecipients(users []entities.User) (map[int]struct{}, map[string]struct{}) {
	groups := make(map[int]struct{})
	teachers := make(map[string]struct{})
	for _, u := range users {
		if u.Group > 0 {
			groups[u.Group] = struct{}{}
		} else if u.TeacherName != "" {
			teachers[u.TeacherName] = struct{}{}
		}
	}
	return groups, teachers
}

func (a *App) notifyGroupsAfterLessons(
	ctx context.Context,
	localTime time.Time,
	dayIdx int,
	loc *time.Location,
	groups map[int]struct{},
	sentGroups map[int]bool,
	sentUsers map[int64]bool,
) {
	for groupID := range groups {
		if sentGroups[groupID] {
			continue
		}

		grp, err := a.scheduleSvc.GetGroupLatestSchedule(ctx, int32(groupID))
		if err != nil || dayIdx >= len(grp.Days) {
			continue
		}

		lastSubject := formatter.FindLastSubject(grp.Days[dayIdx].Subjects)
		if lastSubject == -1 {
			continue
		}

		endTime, ok := formatter.GetDayEndTime(dayIdx, lastSubject+1, loc)
		if !ok || localTime.Before(endTime) {
			continue
		}

		sentGroups[groupID] = true
		a.log.Info("Lessons ended for group, sending next day digest", "group", groupID, "time", localTime.Format("15:04:05"))
		sentIDs, sendErr := a.notifier.SendNextDayDigestForTarget(ctx, a.scheduleSvc, a.teacherSvc, groupID, "")
		if sendErr != nil {
			a.log.Warn("failed sending next day digest for group", "group", groupID, "error", sendErr)
		}
		for _, id := range sentIDs {
			sentUsers[id] = true
		}
	}
}

func (a *App) notifyTeachersAfterLessons(
	ctx context.Context,
	localTime time.Time,
	dayIdx int,
	loc *time.Location,
	teachers map[string]struct{},
	sentTeachers map[string]bool,
	sentUsers map[int64]bool,
) {
	for teacherName := range teachers {
		if sentTeachers[teacherName] {
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

		sentTeachers[teacherName] = true
		a.log.Info("Lessons ended for teacher, sending next day digest", "teacher", teacherName, "time", localTime.Format("15:04:05"))
		sentIDs, sendErr := a.notifier.SendNextDayDigestForTarget(ctx, a.scheduleSvc, a.teacherSvc, 0, teacherName)
		if sendErr != nil {
			a.log.Warn("failed sending next day digest for teacher", "teacher", teacherName, "error", sendErr)
		}
		for _, id := range sentIDs {
			sentUsers[id] = true
		}
	}
}
