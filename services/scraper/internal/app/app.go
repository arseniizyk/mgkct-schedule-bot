package app

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	parserUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/parser/usecase"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/schedule/usecase"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database/postgre"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/server"
	"google.golang.org/grpc"
)

type App struct {
	cfg *config.Config
	lis net.Listener
	db  database.DatabaseRepository
}

func New() (*App, error) {
	cfg, err := config.New()
	if err != nil {
		slog.Error("can't load cfg", "err", err)
		return nil, err
	}

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("can't load gRPC listener", "err", err)
		return nil, err
	}

	return &App{
		cfg: cfg,
		lis: lis,
		db:  initDB(),
	}, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	parserObj := parser.New()
	parserUC := parserUC.NewParserUsecase(a.db, parserObj)

	scheduleUC := scheduleUC.NewScheduleUsecase(a.db)

	go func() {
		ch := parserUC.GetScheduleEvery(ctx, 1*time.Minute)
		for sch := range ch {
			scheduleUC.SaveToCache(sch)
			slog.Info("New schedule parsed and saved")
		}
	}()

	grpcServer := grpc.NewServer()
	httpSrv := server.NewHTTPServer(scheduleUC, a.cfg.HttpPort)
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
		slog.Info("gRPC server started", "port", a.cfg.GRPCPort)
		if err := grpcServer.Serve(a.lis); err != nil {
			slog.Error("gRPC serve error", "err", err)
		}
	}()
}

func initDB() database.DatabaseRepository {
	url := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
		os.Getenv("POSTGRES_SSL"),
	)

	db, err := postgre.NewDatabase(url)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}

	return db
}
