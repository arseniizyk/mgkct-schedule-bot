package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/arseniizyk/mgkct-schedule-bot/libs/database"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/nats-io/nats.go"

	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/usecase"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/bus"
	tbot "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/keyboard"
	tbotRepo "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state/memory"
	tbotUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/usecase"
	tele "gopkg.in/telebot.v4"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	db  *database.Database
	bot *tele.Bot
	h   *tbot.Handler
	nc  *nats.Conn
	cfg *config.Config
}

func New(cfg *config.Config) (*App, error) {
	db, err := database.New(cfg)
	if err != nil {
		slog.Error("connect db", "err", err)
		return nil, err
	}

	if err := db.Ping(context.Background()); err != nil {
		slog.Error("bad ping db", "err", err)
		return nil, err
	}

	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("connect nats", "err", err)
		return nil, err
	}

	pref := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		slog.Error("error creating bot", "err", err)
		return nil, err
	}

	return &App{
		db:  db,
		bot: b,
		nc:  nc,
		cfg: cfg,
	}, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	grpcConn, err := grpc.NewClient(a.cfg.ScraperURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect GRPC Server", "url", a.cfg.ScraperURL, "err", err)
		return err
	}

	scheduleUC := scheduleUC.New(pb.NewScheduleServiceClient(grpcConn))

	userRepo := tbotRepo.New(a.db.Pool)
	userUC := tbotUC.NewUserUsecase(scheduleUC, userRepo)
	sm := memory.NewMemory()

	a.h = tbot.NewHandler(userUC, sm, a.bot)

	b := bus.New(tbotUC.NewScheduleHandlerUsecase(a.h, userRepo), a.nc)
	b.SubscribeGroupUpdates()

	a.StartBot()

	return nil
}

func (a *App) StartBot() {
	a.bot.Use(a.h.LogMessages)

	a.bot.Handle(tele.OnText, a.h.HandleState)
	a.bot.Handle("/start", a.h.Start)
	a.bot.Handle("/setgroup", a.h.SetGroup)
	a.bot.Handle("/group", a.h.Day)
	a.bot.Handle("/week", a.h.Week)
	a.bot.Handle("/day", a.h.Day)
	a.bot.Handle("/calls", a.h.Calls)
	a.bot.Handle("/cancel", a.h.Cancel)
	a.bot.Handle(tele.OnCallback, a.h.HandleCallback)
	a.bot.Handle(kbd.BtnDay, a.h.Day)
	a.bot.Handle(kbd.BtnCalls, a.h.Calls)
	a.bot.Handle(kbd.BtnWeek, a.h.Week)

	slog.Info("Bot started!", "username", a.bot.Me.Username)

	a.bot.Start()
}
