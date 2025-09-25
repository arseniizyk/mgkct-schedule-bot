package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	scheduleRepo "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository/postgres"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/usecase"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/service"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/transport"
	"google.golang.org/grpc"
)

type App struct {
	lis        net.Listener
	db         *database.Database
	grpcServer *grpc.Server
	parserSvc  *service.ParserService
	scheduleUC schedule.ScheduleUsecase
	nc         *transport.Nats
}

func New(cfg *config.Config) (*App, error) {
	lis, err := net.Listen("tcp", ":"+"9001")
	if err != nil {
		return nil, fmt.Errorf("start net listener: %w", err)
	}

	db, err := database.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	if err := db.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	nc, err := transport.NewNats(cfg)
	if err != nil {
		return nil, fmt.Errorf("can't connect to NATS")
	}

	parserObj := parser.New()
	scheduleRepo := scheduleRepo.New(db.Pool)
	scheduleUC := scheduleUC.New(scheduleRepo)
	parserSvc := service.NewParserService(scheduleUC, parserObj, nc)

	return &App{
		lis:        lis,
		db:         db,
		grpcServer: grpc.NewServer(),
		parserSvc:  parserSvc,
		scheduleUC: scheduleUC,
		nc:         nc,
	}, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		scheduleCh := a.parserSvc.ParseScheduleEvery(ctx, time.Minute)
		for sch := range scheduleCh {
			a.scheduleUC.SaveToCache(sch)
			slog.Info("New schedule parsed and saved")
		}
	}()

	go func() {
		pb.RegisterScheduleServiceServer(a.grpcServer, transport.NewGRPCServer(a.scheduleUC))
		slog.Info("gRPC server started", "address", a.lis.Addr().String())
		if err := a.grpcServer.Serve(a.lis); err != nil {
			slog.Error("gRPC serve error", "err", err)
		}
	}()

	return a.shutdown(cancel)
}

func (a *App) shutdown(cancel context.CancelFunc) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	defer signal.Stop(sigChan)
	defer cancel()

	<-sigChan

	a.nc.Close()
	a.db.Close()
	a.grpcServer.GracefulStop()
	return nil
}
