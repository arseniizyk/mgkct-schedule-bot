package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/db"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/dictionary"
	httpserver "github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/renderer"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/middleware"
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
	stateMgr    *state.MemoryManager
	scraper     *service.ScraperService
	notifier    *notifier.Notifier
	eventCh     chan service.Event
	httpSrv     *httpserver.Server
	scheduleSvc *service.ScheduleService
	teacherSvc  *service.TeacherService
	digestRepo  *repository.DigestSentRepository
	userRepo    *repository.UserRepository
	actWorker   *middleware.ActivityWorker
}

func New(log *slog.Logger, cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	database, err := db.Connect(ctx, &cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("app: database connect: %w", err)
	}

	scheduleRepo := repository.NewScheduleRepository(database)
	teacherRepo := repository.NewTeacherScheduleRepository(database)
	facultyRepo := repository.NewFacultyRepository(database)
	userRepo := repository.NewUserRepository(database)
	digestRepo := repository.NewDigestSentRepository(database)

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
	scraperSvc.SetTeacherCache(teacherSvc)
	scraperSvc.SetAnalyticsCache(analyticsSvc)

	botPref, webhookHandler := telegram.BuildBotPref(cfg)

	bot, err := tele.NewBot(botPref)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("app: create bot: %w", err)
	}

	stateMgr := state.NewMemoryManager()
	actWorker := middleware.NewActivityWorker(log, userRepo)

	bot.Use(middleware.ActivityMiddleware(actWorker))
	bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Message() != nil && strings.HasPrefix(c.Message().Text, "/") {
				_ = stateMgr.Clear(c.Chat().ID)
			}
			return next(c)
		}
	})

	telegram.RegisterHandlers(
		bot,
		log,
		cfg,
		scheduleSvc,
		teacherSvc,
		analyticsSvc,
		userRepo,
		stateMgr,
		subjectDict,
	)

	broadcastLimiter := rate.NewLimiter(rate.Limit(25), 1)
	handlers.SetBroadcastLimiter(broadcastLimiter)

	eventCh := make(chan service.Event, 100)
	notif := notifier.New(log, bot, userRepo, cfg.Bot.AdminIDs, broadcastLimiter)

	httpSrv := httpserver.New(
		log,
		cfg,
		database,
		scheduleSvc,
		teacherSvc,
		analyticsSvc,
		userRepo,
	)

	// Если выбран webhook-режим — монтируем обработчик в HTTP-роутер
	if webhookHandler != nil {
		const webhookPath = "/telegram/webhook"
		httpSrv.RegisterWebhook(webhookPath, webhookHandler)
		log.Info("Бот запущен в Webhook-режиме", "path", webhookPath)
	} else {
		log.Info("Бот запущен в Long Polling-режиме")
	}

	return &App{
		log:         log,
		cfg:         cfg,
		database:    database,
		bot:         bot,
		stateMgr:    stateMgr,
		scraper:     scraperSvc,
		notifier:    notif,
		eventCh:     eventCh,
		httpSrv:     httpSrv,
		scheduleSvc: scheduleSvc,
		teacherSvc:  teacherSvc,
		digestRepo:  digestRepo,
		userRepo:    userRepo,
		actWorker:   actWorker,
	}, nil
}

func (a *App) Run() error {
	log := a.log.With("operation", "app.App.Run")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers.SetBroadcastContext(ctx)

	var workersWg sync.WaitGroup

	workersWg.Go(func() {
		a.listenEvents(ctx)
	})

	// Сидируем лог дайджеста перед запуском шедулера:
	a.seedDigestLogIfNeeded(ctx)

	workersWg.Go(func() {
		a.startEveningDigestScheduler(ctx)
	})

	workersWg.Go(func() {
		a.scraper.Start(ctx, a.eventCh)
	})

	workersWg.Go(func() {
		a.stateMgr.StartCleanup(ctx, 5*time.Minute)
	})

	workersWg.Go(func() {
		a.actWorker.Run(ctx)
	})

	go func() {
		if err := a.httpSrv.Start(); err != nil {
			log.Error("http server failed", "error", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	shutdownDone := make(chan struct{})
	var shutdownOnce sync.Once

	shutdown := func() {
		shutdownOnce.Do(func() {
			log.Info("Shutting down bot gracefully...")

			a.bot.Stop()
			cancel()

			shutdownCtx, httpCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer httpCancel()

			if err := a.httpSrv.Shutdown(shutdownCtx); err != nil {
				log.Error("failed to gracefully stop http server", "error", err)
			}

			workersDone := make(chan struct{})
			go func() {
				workersWg.Wait()
				close(workersDone)
			}()

			select {
			case <-workersDone:
				log.Info("All background workers stopped")
			case <-time.After(3 * time.Second):
				log.Warn("Timed out waiting for background workers to stop")
			}

			if a.database != nil {
				if err := a.database.Close(); err != nil {
					log.Error("failed to close database", "error", err)
				}
			}

			close(shutdownDone)
		})
	}

	go func() {
		select {
		case <-sigChan:
			shutdown()
		case <-shutdownDone:
		}
	}()

	log.Info("Bot started!", "username", a.bot.Me.Username)
	a.bot.Start()

	shutdown()
	<-shutdownDone

	log.Info("Application stopped cleanly")
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
				renderer.GetDefaultCache().Clear()
				if err := a.notifier.HandleNewWeek(ctx, event.Week); err != nil {
					log.Error("failed broadcasting new week", "error", err)
				}
			case service.EventGroupScheduleUpdated:
				log.Info("Received EventGroupScheduleUpdated", "group_id", event.Group.ID)
				if event.Group != nil {
					renderer.GetDefaultCache().InvalidateGroup(event.Group.ID)
				}
				if err := a.notifier.HandleGroupScheduleUpdate(ctx, event.Group, event.Diff); err != nil {
					log.Error("failed broadcasting group update", "group_id", event.Group.ID, "error", err)
				}
			case service.EventTeacherScheduleUpdated:
				log.Info("Received EventTeacherScheduleUpdated", "name", event.Teacher.Name)
				if event.Teacher != nil {
					renderer.GetDefaultCache().InvalidateTeacher(event.Teacher.Name)
				}
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
