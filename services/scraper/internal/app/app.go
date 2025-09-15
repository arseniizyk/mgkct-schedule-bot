package app

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	parserUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/parser/usecase"
	scheduleRepo "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/schedule/repository/postgres"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/schedule/usecase"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/server"
	"google.golang.org/grpc"
)

type App struct {
	lis net.Listener
	db  *database.Database
}

func New() (*App, error) {
	if err := config.LoadEnv(); err != nil {
		slog.Error("can't load cfg", "err", err)
		return nil, err
	}

	lis, err := net.Listen("tcp", ":"+os.Getenv("GRPC_PORT"))
	if err != nil {
		slog.Error("can't start net listener for GRPC", "err", err)
		return nil, err
	}

	db, err := database.New()
	if err != nil {
		slog.Error("can't connect to database", "err", err)
		return nil, err
	}

	if err := db.Ping(context.Background()); err != nil {
		slog.Error("bad ping to DB", "err", err)
		return nil, err
	}

	return &App{
		lis: lis,
		db:  db,
	}, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	parserObj := parser.New()
	scheduleRepo := scheduleRepo.New(a.db.Pool)
	scheduleUC := scheduleUC.New(scheduleRepo)
	parserUC := parserUC.New(scheduleUC, parserObj)

	go func() {
		scheduleCh := parserUC.ParseScheduleEvery(ctx, 1*time.Minute)
		for sch := range scheduleCh {
			scheduleUC.SaveToCache(sch)
			slog.Info("New schedule parsed and saved")
		}
	}()

	grpcServer := grpc.NewServer()
	httpSrv := server.NewHTTPServer(scheduleUC)
	a.startGRPC(scheduleUC, grpcServer)
	httpSrv.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	slog.Info("Shutting down...")

	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelTimeout()
	cancel()

	httpSrv.Shutdown(ctxTimeout)
	grpcServer.GracefulStop()

	slog.Info("clean shutdown")
	return nil
}

func (a *App) startGRPC(schUC *scheduleUC.ScheduleUsecase, grpcServer *grpc.Server) {
	pb.RegisterScheduleServiceServer(grpcServer, server.NewGRPCServer(schUC))

	go func() {
		slog.Info("gRPC server started", "port", "GRPC_PORT")
		if err := grpcServer.Serve(a.lis); err != nil {
			slog.Error("gRPC serve error", "err", err)
		}
	}()
}
