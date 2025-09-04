package main

import (
	"log"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v4"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	// conn, err := grpc.NewClient("localhost:"+cfg.GRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	// if err != nil {
	// 	log.Fatalf("failed to connect: %v", err)
	// }

	// scheduleStub := transport.New(conn)
	// scheduleSvc := scheduleUC.NewScheduleUseCase(scheduleStub)
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	// schedule, err := scheduleSvc.GetGroupSchedule(ctx, 88)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	pref := tele.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	userRepo := postgres.NewUserRepo(&pgxpool.Pool{}) // TODO: init pgx pool
	userUC := usecase.NewUserUseCase(userRepo)
	h := delivery.NewHandler(userUC)

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
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
}
