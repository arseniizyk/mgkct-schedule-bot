package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/arseniizyk/mgkct-schedule-bot/libs/database"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/keyboard"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	tele "gopkg.in/telebot.v4"
)

type App struct {
	diContainer *diContainer
	db          *database.Database
	bot         *tele.Bot
	h           *bot.Handler
	grpcConn    *grpc.ClientConn
	nc          *nats.Conn
	cfg         *config.Config
}

func New(cfg *config.Config) (*App, error) {
	a := &App{
		cfg: cfg,
	}

	pref := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		slog.Error("error creating bot", "err", err)
		return nil, err
	}

	a.bot = bot

	if err := a.initDeps(); err != nil {
		slog.Error("error initializing DI", "err", err)
		return nil, err
	}

	return a, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	a.h = a.diContainer.TelegramBotHandler()
	if err := a.subscribeScheduleUpdates(); err != nil {
		slog.Error("subscribe to schedule updates failed", "err", err)
		return err
	}

	return a.StartBot()
}

func (a *App) subscribeScheduleUpdates() error {
	_, err := a.nc.Subscribe("schedule.updates", func(msg *nats.Msg) {
		group := &pb.GroupScheduleResponse{}
		err := proto.Unmarshal(msg.Data, group)
		if err != nil {
			slog.Error("unmarshalling proto", "err", err)
		}
		slog.Info("Schedule updated", "group_id", group.Group.Id)

		if err := a.diContainer.TelegramBotHandler().HandleScheduleUpdate(context.Background(), group); err != nil {
			slog.Error("handle schedule update", "err", err)
		}
	})
	return err
}

func (a *App) StartBot() error {
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	defer signal.Stop(sigChan)

	<-sigChan

	a.db.Close()
	a.nc.Close()

	if err := a.grpcConn.Close(); err != nil {
		slog.Error("can't close gRPCconnection", "err", err)
		return err
	}

	return nil
}

func (a *App) initDeps() error {
	inits := []func() error{
		a.initDB,
		a.initNATS,
		a.initGRPC,
		a.initDI,
	}

	for _, f := range inits {
		if err := f(); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) initDB() error {
	var err error
	a.db, err = database.New(a.cfg)
	if err != nil {
		slog.Error("can't connect to database")
		return err
	}
	if err := a.db.Ping(context.Background()); err != nil {
		slog.Error("Database ping error", "err", err)
		return err
	}
	return err
}

func (a *App) initNATS() error {
	var err error
	a.nc, err = nats.Connect(a.cfg.NatsURL, nats.Name("tg-bot"))
	if err != nil {
		slog.Error("can't connect NATS", "url", a.cfg.NatsURL, "err", err)
		return err
	}
	return err
}

func (a *App) initDI() error {
	a.diContainer = NewDIContainer(a.nc, a.db.Pool, a.grpcConn, a.bot)
	return nil
}

func (a *App) initGRPC() error {
	var err error
	a.grpcConn, err = grpc.NewClient(a.cfg.ScraperURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect GRPC Server", "url", a.cfg.ScraperURL, "err", err)
		return err
	}

	return nil
}
