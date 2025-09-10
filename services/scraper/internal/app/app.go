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

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database/postgre"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/server"
	"google.golang.org/grpc"
)

type App struct {
	cfg *config.Config
	lis net.Listener
	db  database.DatabaseUseCase
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
	var schedule *models.Schedule

	defer a.db.Close()

	parser := parser.New(a.db)

	go func() {
		tick := time.NewTicker(1 * time.Minute)

		for range tick.C {
			start := time.Now()
			slog.Info("parsing")

			sch, week, err := parser.Parse()
			if err != nil {
				slog.Error("parsing error:", "err", err)
			}

			if sch != schedule {
				schedule = sch
				if err := a.db.SaveSchedule(context.Background(), *week, schedule); err != nil {
					slog.Error("can't save schedule to database", "err", err)
					continue
				}
			}

			slog.Info("parsed", "duration", time.Since(start))
		}
	}()

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	httpSrv := server.NewHTTPServer(a.db, a.cfg.HttpPort)
	grpcServer := grpc.NewServer()

	a.startGRPC(a.db, grpcServer)
	httpSrv.Start()

	<-sigChan
	slog.Info("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	httpSrv.Shutdown(ctx)
	grpcServer.GracefulStop()
	a.db.Close()

	slog.Info("clean shutdown")

	return nil
}

func (a *App) startGRPC(db database.DatabaseUseCase, grpcServer *grpc.Server) {
	pb.RegisterScheduleServiceServer(grpcServer, server.NewGRPCServer(db))

	go func() {
		slog.Info("gRPC server started", "port", a.cfg.GRPCPort)
		if err := grpcServer.Serve(a.lis); err != nil {
			slog.Error("gRPC serve error", "err", err)
		}
	}()
}

func initDB() database.DatabaseUseCase {
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
