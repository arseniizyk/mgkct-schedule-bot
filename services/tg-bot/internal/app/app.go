package app

import (
	"context"
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

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/infrastructure/db"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/infrastructure/logger"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/repository"
	userRepository "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/repository/user"
	scheduleTransport "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/grpc/schedule"
	teacherScheduleTransport "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/grpc/teacher_schedule"
	weekTransport "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/grpc/week"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/handlers"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/notifier"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/state"
	scheduleUsecase "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/usecases/schedule"
	teacherScheduleUsecase "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/usecases/teacher_schedule"
	weekUsecase "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/usecases/week"
)

type TelegramNotifier interface {
	HandleScheduleUpdate(ctx context.Context, resp *pb.GroupScheduleResponse) error
	HandleWeekUpdate(ctx context.Context) error
	HandleTeacherScheduleUpdate(ctx context.Context, resp *pb.TeacherScheduleResponse) error
	HandleTeacherWeekUpdate(ctx context.Context) error
}

type App struct {
	cfg *config.Config
	log *slog.Logger

	pool     *pgxpool.Pool
	grpcConn *grpc.ClientConn
	natsConn *nats.Conn

	userRepository   repository.User
	telegramNotifier TelegramNotifier

	healthSrv *http.Server

	bot *tele.Bot
}

func New(log *slog.Logger, cfg *config.Config) (*App, error) {
	a := &App{
		log: log,
		cfg: cfg,
	}

	pref := tele.Settings{
		Token:  cfg.Bot.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Error("error creating bot", "error", err)
		return nil, err
	}

	if err := a.initDeps(); err != nil {
		log.Error("can't init dependencies", "error", err)
		return nil, err
	}

	a.bot = bot

	a.userRepository = userRepository.New(a.pool)
	a.telegramNotifier = notifier.New(log, bot, a.userRepository)

	return a, nil
}

func (a *App) Run() error {
	log := a.log.With("operation", "app.App.Run")
	ctx := context.Background()

	if err := a.subscribeScheduleUpdates(ctx); err != nil {
		slog.Error("subscribe to schedule updates failed", "err", err)
		return err
	}
	if err := a.subscribeWeekUpdates(ctx); err != nil {
		slog.Error("subscribe to week updates failed", "err", err)
		return err
	}
	if err := a.subscribeTeacherScheduleUpdates(ctx); err != nil {
		slog.Error("subscribe to teacher schedule updates failed", "err", err)
		return err
	}
	if err := a.subscribeTeacherWeekUpdates(ctx); err != nil {
		slog.Error("subscribe to teacher week updates failed", "err", err)
		return err
	}

	defer a.natsConn.Close()
	defer a.pool.Close()

	a.healthSrv = newHealthServer(a.pool, a.cfg.Bot.HealthPort)

	go func() {
		if err := a.healthSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server failed", "err", err)
		}
	}()

	return a.StartBot()
}

func (a *App) StartBot() error {
	scraperService := pb.NewScheduleServiceClient(a.grpcConn)

	weekTransport := weekTransport.New(a.log, a.natsConn, scraperService)
	weekUsecase := weekUsecase.New(a.log, weekTransport)

	scheduleTransport := scheduleTransport.New(a.log, a.natsConn, scraperService)
	scheduleUsecase := scheduleUsecase.New(a.log, a.userRepository, scheduleTransport)

	teacherScheduleGrpcTransport := teacherScheduleTransport.New(a.log, a.natsConn, scraperService)
	teacherScheduleUsecase := teacherScheduleUsecase.New(a.log, a.userRepository, teacherScheduleGrpcTransport)

	stateManager := state.NewDB(a.userRepository)

	log := a.log.With(
		"operation", "app.App.StartBot",
	)

	a.bot.Use(logger.LogMessages(a.log))

	// Любая команда (текст, начинающийся с "/") сбрасывает ожидающее состояние
	// (WaitingGroup/WaitingTeacher), чтобы случайный текст после неё не был принят
	// за ввод группы/преподавателя.
	a.bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Message() != nil && strings.HasPrefix(c.Message().Text, "/") {
				_ = stateManager.Clear(c.Chat().ID)
			}
			return next(c)
		}
	})

	a.bot.Handle("/start", handlers.Start(a.log, a.userRepository))
	a.bot.Handle("/calls", handlers.Calls())
	a.bot.Handle("/setgroup", handlers.SetGroup(log, a.userRepository, stateManager))
	a.bot.Handle("/group", handlers.Day(a.log, scheduleUsecase))
	a.bot.Handle("/week", handlers.Week(a.log, scheduleUsecase, weekUsecase))
	a.bot.Handle("/day", handlers.Day(a.log, scheduleUsecase))
	a.bot.Handle("/cancel", handlers.Cancel(log, stateManager))
	a.bot.Handle("/teachers", handlers.Teachers(a.log, teacherScheduleUsecase))
	a.bot.Handle("/setteacher", handlers.SetTeacher(a.log, a.userRepository, teacherScheduleUsecase, stateManager))
	a.bot.Handle("/teacher", handlers.TeacherDay(a.log, teacherScheduleUsecase))
	a.bot.Handle(kbd.BtnDay, handlers.Day(log, scheduleUsecase))
	a.bot.Handle(kbd.BtnCalls, handlers.Calls())
	a.bot.Handle(kbd.BtnWeek, handlers.Week(a.log, scheduleUsecase, weekUsecase))
	a.bot.Handle(kbd.BtnTeachers, handlers.Teachers(a.log, teacherScheduleUsecase))
	a.bot.Handle(tele.OnCallback, handlers.CallbacksHandler(a.log, scheduleUsecase, weekUsecase, teacherScheduleUsecase, teacherScheduleUsecase))
	a.bot.Handle(tele.OnText, handlers.StatesHandler(a.log, a.userRepository, stateManager, stateManager, a.userRepository, teacherScheduleUsecase))

	log.Info("Bot started!", "username", a.bot.Me.Username)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		log.Info("shutting down")
		a.bot.Stop()
	}()
	defer func() {
		signal.Stop(sigChan)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.healthSrv.Shutdown(shutdownCtx); err != nil {
			log.Error("failed to shutdown health server", "err", err)
		}
	}()

	a.bot.Start()

	if err := a.grpcConn.Close(); err != nil {
		return fmt.Errorf("failed to close grpc connection: %w", err)
	}
	return nil
}

func (a *App) subscribeScheduleUpdates(ctx context.Context) error {
	logger := a.log.With("operation", "app.subscribeScheduleUpdates")

	_, err := a.natsConn.Subscribe("schedule.updates", func(msg *nats.Msg) {
		var resp pb.GroupScheduleResponse

		if err := proto.Unmarshal(msg.Data, &resp); err != nil {
			logger.ErrorContext(ctx, "failed to unmarshal proto", "error", err)
			return
		}

		logger.InfoContext(ctx, "Schedule updated", "group_id", resp.GetGroup().GetId())

		handleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := a.telegramNotifier.HandleScheduleUpdate(handleCtx, &resp); err != nil {
			logger.ErrorContext(ctx, "failed to handle schedule update", "err", err)
		}
	})

	return err
}

func (a *App) subscribeWeekUpdates(ctx context.Context) error {
	logger := a.log.With("operation", "app.subscribeWeekUpdates")

	_, err := a.natsConn.Subscribe("schedule.week.updates", func(msg *nats.Msg) {
		logger.InfoContext(ctx, "Week updated", "date", msg.Data)

		handleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := a.telegramNotifier.HandleWeekUpdate(handleCtx); err != nil {
			logger.ErrorContext(ctx, "failed to handle week update", "error", err)
		}
	})
	return err
}

func (a *App) subscribeTeacherScheduleUpdates(ctx context.Context) error {
	logger := a.log.With("operation", "app.subscribeTeacherScheduleUpdates")

	_, err := a.natsConn.Subscribe("teacher_schedule.updates", func(msg *nats.Msg) {
		var resp pb.TeacherScheduleResponse

		if err := proto.Unmarshal(msg.Data, &resp); err != nil {
			logger.ErrorContext(ctx, "failed to unmarshal proto", "error", err)
			return
		}

		logger.InfoContext(ctx, "Teacher schedule updated", "name", resp.GetTeacher().GetName())

		handleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := a.telegramNotifier.HandleTeacherScheduleUpdate(handleCtx, &resp); err != nil {
			logger.ErrorContext(ctx, "failed to handle teacher schedule update", "err", err)
		}
	})

	return err
}

func (a *App) subscribeTeacherWeekUpdates(ctx context.Context) error {
	logger := a.log.With("operation", "app.subscribeTeacherWeekUpdates")

	_, err := a.natsConn.Subscribe("teacher_schedule.week.updates", func(msg *nats.Msg) {
		logger.InfoContext(ctx, "Teacher week updated", "date", msg.Data)

		handleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := a.telegramNotifier.HandleTeacherWeekUpdate(handleCtx); err != nil {
			logger.ErrorContext(ctx, "failed to handle teacher week update", "error", err)
		}
	})
	return err
}

func (a *App) initDeps() error {
	inits := []func() error{
		a.initDB,
		a.initNATS,
		a.initGRPC,
	}

	for _, init := range inits {
		if err := init(); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) initDB() error {
	ctx := context.Background()

	pool, err := db.Connect(ctx, &a.cfg.Bot.DB)
	if err != nil {
		return fmt.Errorf("can't connect to database: %w", err)
	}
	a.pool = pool

	if err := a.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database error: %w", err)
	}

	return nil
}

func (a *App) initNATS() error {
	var err error

	a.natsConn, err = nats.Connect(a.cfg.Nats.URL, nats.Name("tg-bot"))
	if err != nil {
		return fmt.Errorf("can't connect NATS: %s : %w", a.cfg.Nats.URL, err)
	}

	return nil
}

func (a *App) initGRPC() error {
	var err error

	url := net.JoinHostPort("scraper", a.cfg.Scraper.GRPCPort)
	a.grpcConn, err = grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect grpc server: %s : %w", url, err)
	}

	return nil
}
