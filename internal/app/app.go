package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/db"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/notifier"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type App struct {
	log       *slog.Logger
	cfg       *config.Config
	database  *sql.DB
	bot       *tele.Bot
	scraper   *service.ScraperService
	notifier  *notifier.Notifier
	eventCh   chan service.Event
	healthSrv *http.Server
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
	userRepo := repository.NewUserRepository(database)

	// 3. Сервисы
	scheduleSvc := service.NewScheduleService(log, scheduleRepo)
	teacherSvc := service.NewTeacherService(log, teacherRepo)

	studentParser := parser.New(log)
	teacherParser := parser.NewTeacherParser(log)

	scraperSvc := service.NewScraperService(
		log,
		cfg.Scraper,
		studentParser,
		teacherParser,
		scheduleRepo,
		teacherRepo,
		scheduleSvc,
	)

	// 4. Telegram Бот
	botPref := tele.Settings{
		Token:  cfg.Bot.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
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

	// Регистрация команд
	bot.Handle("/start", handlers.Start(log, userRepo))
	bot.Handle("/calls", handlers.Calls())
	bot.Handle("/cancel", handlers.Cancel(log, stateMgr))
	bot.Handle("/setgroup", handlers.SetGroup(log, userRepo, stateMgr))
	bot.Handle("/group", handlers.Day(log, scheduleSvc, userRepo))
	bot.Handle("/day", handlers.Day(log, scheduleSvc, userRepo))
	bot.Handle("/week", handlers.Week(log, scheduleSvc, userRepo))
	bot.Handle("/teachers", handlers.Teachers(log, teacherSvc, stateMgr))
	bot.Handle("/setteacher", handlers.SetTeacher(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/teacher", handlers.TeacherDay(log, teacherSvc, userRepo))
	bot.Handle("/teacherweek", handlers.TeacherWeek(log, teacherSvc, userRepo))

	// Кнопки Reply-клавиатуры
	bot.Handle(kbd.BtnDay, handlers.Day(log, scheduleSvc, userRepo))
	bot.Handle(kbd.BtnWeek, handlers.Week(log, scheduleSvc, userRepo))
	bot.Handle(kbd.BtnCalls, handlers.Calls())
	bot.Handle(kbd.BtnTeachers, handlers.Teachers(log, teacherSvc, stateMgr))

	// Инлайн-коллбэки и ввод текста
	bot.Handle(tele.OnCallback, handlers.CallbacksHandler(log, scheduleSvc, teacherSvc))
	bot.Handle(tele.OnText, handlers.StatesHandler(log, userRepo, stateMgr, teacherSvc))

	eventCh := make(chan service.Event, 100)
	notif := notifier.New(log, bot, userRepo)

	return &App{
		log:      log,
		cfg:      cfg,
		database: database,
		bot:      bot,
		scraper:  scraperSvc,
		notifier: notif,
		eventCh:  eventCh,
	}, nil
}

func (a *App) Run() error {
	log := a.log.With("operation", "app.App.Run")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Запуск обработчика событий рассылки
	go a.listenEvents(ctx)

	// 2. Запуск фонового скрапера
	go a.scraper.Start(ctx, a.eventCh)

	// 3. Запуск Healthcheck HTTP сервера
	a.startHealthServer()

	// 4. Ожидание сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Shutting down gracefully...")
		a.bot.Stop()
		cancel()

		if a.healthSrv != nil {
			shutdownCtx, sCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer sCancel()
			_ = a.healthSrv.Shutdown(shutdownCtx)
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
				if err := a.notifier.HandleGroupScheduleUpdate(ctx, event.Group); err != nil {
					log.Error("failed broadcasting group update", "group_id", event.Group.ID, "error", err)
				}
			case service.EventTeacherScheduleUpdated:
				log.Info("Received EventTeacherScheduleUpdated", "name", event.Teacher.Name)
				if err := a.notifier.HandleTeacherScheduleUpdate(ctx, event.Teacher); err != nil {
					log.Error("failed broadcasting teacher update", "name", event.Teacher.Name, "error", err)
				}
			}
		}
	}
}

func (a *App) startHealthServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := a.database.PingContext(pingCtx); err != nil {
			http.Error(w, "database unhealthy: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	addr := net.JoinHostPort("0.0.0.0", a.cfg.Bot.HealthPort)
	a.healthSrv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := a.healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Error("health server failed", "error", err)
		}
	}()
}
