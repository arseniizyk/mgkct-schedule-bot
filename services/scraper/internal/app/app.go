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
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/server"
	"google.golang.org/grpc"
)

type App struct {
	cfg      *config.Config
	crawlSvc *crawler.Crawler
	lis      net.Listener
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

	crawlSvc := crawler.New()

	return &App{
		cfg:      cfg,
		crawlSvc: crawlSvc,
		lis:      lis,
	}, nil
}

func (a *App) Run() error {
	schedule, err := a.crawlSvc.Crawl()
	if err != nil {
		slog.Error("crawing error:", "err", err)
		return err
	}

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	httpSrv := server.NewHTTPServer(schedule, a.cfg.HttpPort)
	grpcServer := grpc.NewServer()
	
	a.startGRPC(schedule, grpcServer)
	httpSrv.Start()

	<-sigChan
	slog.Info("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	httpSrv.Shutdown(ctx)
	grpcServer.GracefulStop()

	slog.Info("clean shutdown")

	return nil
}

func (a *App) startGRPC(schedule *models.Schedule, grpcServer *grpc.Server) {
	pb.RegisterScheduleServiceServer(grpcServer, server.NewGRPCServer(schedule))

	go func() {
		slog.Info("gRPC server started", "port", a.cfg.GRPCPort)
		if err := grpcServer.Serve(a.lis); err != nil {
			slog.Error("gRPC serve error", "err", err)
		}
	}()
}
