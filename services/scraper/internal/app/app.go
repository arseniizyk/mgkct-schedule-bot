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

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule"
	scheduleRepo "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository/postgres"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/usecase"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/usecase"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/parser"
	server "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/transport"
	"google.golang.org/grpc"
)

type App struct {
	lis        net.Listener
	db         *database.Database
	grpcServer *grpc.Server
	httpSrv    *server.HTTPServer
	parserUC   usecase.ParserUsecase
	scheduleUC schedule.ScheduleUsecase
}

func New() (*App, error) {
	if err := config.LoadEnv(); err != nil {
		return nil, fmt.Errorf("load cfg: %w", err)
	}

	lis, err := net.Listen("tcp", ":"+os.Getenv("GRPC_PORT"))
	if err != nil {
		return nil, fmt.Errorf("start net listener: %w", err)
	}

	db, err := database.New()
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	if err := db.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	parserObj := parser.New()
	scheduleRepo := scheduleRepo.New(db.Pool)
	scheduleUC := scheduleUC.New(scheduleRepo)
	parserUC := usecase.NewParserUsecase(scheduleUC, parserObj)

	return &App{
		lis:        lis,
		db:         db,
		grpcServer: grpc.NewServer(),
		httpSrv:    server.NewHTTPServer(scheduleUC),
		parserUC:   parserUC,
		scheduleUC: scheduleUC,
	}, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.startParser(ctx)

	a.startGRPC()
	go a.httpSrv.Start()

	a.waitForShutdown(cancel)

	return a.shutdown(ctx)
}

func (a *App) startParser(ctx context.Context) {
	scheduleCh := a.parserUC.ParseScheduleEvery(ctx, time.Minute)
	for sch := range scheduleCh {
		a.scheduleUC.SaveToCache(sch)
		slog.Info("New schedule parsed and saved")
	}
}

func (a *App) startGRPC() {
	pb.RegisterScheduleServiceServer(a.grpcServer, server.NewGRPCServer(a.scheduleUC))

	go func() {
		slog.Info("gRPC server started", "address", a.lis.Addr().String())
		if err := a.grpcServer.Serve(a.lis); err != nil {
			slog.Error("gRPC serve error", "err", err)
		}
	}()
}

func (a *App) waitForShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	<-sigChan
	slog.Info("Shutting down...")
	cancel()
}

func (a *App) shutdown(ctx context.Context) error {
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 2*time.Second)
	defer cancelTimeout()

	if err := a.httpSrv.Shutdown(ctxTimeout); err != nil {
		return err
	}
	a.grpcServer.GracefulStop()

	slog.Info("clean shutdown")
	return nil
}
