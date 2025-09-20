package app

import (
	"context"
	"log/slog"
	"os"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/database"

	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/usecase"

	tbot "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/keyboard"
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
}

func New() (*App, error) {
	if err := config.LoadEnv(); err != nil {
		slog.Error("can't load cfg", "err", err)
		return nil, err
	}

	db, err := database.New()
	if err != nil {
		slog.Error("can't connect to DB", "err", err)
		return nil, err
	}

	if err := db.Ping(context.Background()); err != nil {
		slog.Error("bad ping to DB", "err", err)
		return nil, err
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		slog.Error("Provide telegram token to .env")
		return nil, err
	}

	pref := tele.Settings{
		Token:  token,
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
	}, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	grpcUrl := os.Getenv("GRPC_ADDR")
	if grpcUrl == "" {
		grpcUrl = "localhost:" + os.Getenv("GRPC_PORT")
	}

	conn, err := grpc.NewClient(grpcUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect GRPC Server", "port", os.Getenv("GRPC_PORT"), "err", err)
		return err
	}

	scheduleUC := scheduleUC.New(pb.NewScheduleServiceClient(conn))

	userRepo := tbotRepo.New(a.db.Pool)
	userUC := tbotUC.New(scheduleUC, userRepo)
	sm := memory.NewMemory()

	h := tbot.NewHandler(userUC, sm)
	a.h = h

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
