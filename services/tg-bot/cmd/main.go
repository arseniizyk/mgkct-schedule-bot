package main

import (
	"context"
	"log"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.NewClient("localhost:"+cfg.GRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	scheduleStub := transport.New(conn)
	scheduleSvc := scheduleUC.NewScheduleUseCase(*scheduleStub)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schedule, err := scheduleSvc.GetGroupSchedule(ctx, 88)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(schedule)
}
