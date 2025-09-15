package app

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/database"

	scheduleTransport "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/usecase"

	tbot "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery"
	tbotRepo "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
	tbotUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/usecase"
	tele "gopkg.in/telebot.v4"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	db *database.Database
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

	return &App{
		db: db,
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

	scheduleStub := scheduleTransport.New(conn)
	scheduleUC := scheduleUC.New(scheduleStub)

	pref := tele.Settings{
		Token:  os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	userRepo := tbotRepo.New(a.db.Pool)
	userUC := tbotUC.New(scheduleUC, userRepo)

	h := tbot.NewHandler(userUC)

	b, err := tele.NewBot(pref)
	if err != nil {
		slog.Error("error creating bot", "err", err)
		return err
	}

	b.Use(h.LogMessages)

	b.Handle(tele.OnText, h.HandleState)
	b.Handle("/start", h.Start)
	b.Handle("/setgroup", h.SetGroup)
	b.Handle("/group", h.Group)
	b.Handle("/week", h.Week)
	b.Handle("/day", h.Day)
	b.Handle("/calls", h.Calls)
	b.Handle("/cancel", h.Cancel)

	slog.Info("Bot started!", "username", b.Me.Username)

	b.Start()

	return nil
}
