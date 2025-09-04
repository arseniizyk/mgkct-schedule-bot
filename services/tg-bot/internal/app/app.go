package app

import (
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/config"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/usecase"

	tbotDeliv "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery"
	tbotRepo "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
	tbotUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/usecase"
	tele "gopkg.in/telebot.v4"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	cfg *config.Config
}

func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

func (a *App) Run() error {

	conn, err := grpc.NewClient("localhost:"+a.cfg.GRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect GRPC Server", "port", a.cfg.GRPCPort, "err", err)
		return err
	}

	scheduleStub := transport.New(conn)
	schUC := scheduleUC.NewScheduleUseCase(scheduleStub)

	pref := tele.Settings{
		Token:  a.cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	userRepo := tbotRepo.NewUserRepo(&pgxpool.Pool{}) // TODO: init pgx pool
	userUC := tbotUC.NewUserUseCase(schUC, userRepo)
	h := tbotDeliv.NewHandler(userUC)

	b, err := tele.NewBot(pref)
	if err != nil {
		slog.Error("error creating bot", "err", err)
		return err
	}

	b.Use(h.LogMessages)

	b.Handle("/start", h.Start)
	b.Handle("/setgroup", h.SetGroup)
	b.Handle("/group", h.Group)
	b.Handle("/week", h.Week)
	b.Handle("/day", h.Day)
	b.Handle("/calls", h.Calls)
	b.Handle("/cancel", h.Cancel)

	b.Start()

	return nil
}
