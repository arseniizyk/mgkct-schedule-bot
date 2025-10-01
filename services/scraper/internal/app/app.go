package app

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/arseniizyk/mgkct-schedule-bot/libs/database"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
)

type App struct {
	diContainer *diContainer
	cfg         *config.Config
	lis         net.Listener
	db          *database.Database
	grpcServer  *grpc.Server
	nc          *nats.Conn
}

func New(cfg *config.Config) (*App, error) {
	a := &App{
		cfg:        cfg,
		grpcServer: grpc.NewServer(),
	}

	if err := a.initDeps(); err != nil {
		slog.Error("can't init dependencies", "err", err)
		return nil, err
	}

	return a, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	go func() {
		updatesCh := a.diContainer.ScheduleService().CheckScheduleUpdates(time.Minute)
		for update := range updatesCh {
			if err := a.diContainer.ScheduleTransport().PublishScheduleUpdate(update.Group); err != nil {
				slog.Error("Failed publishing new schedule to NATS")
			}
			slog.Info("New schedule parsed and published to NATS")
		}
	}()

	go func() {
		pb.RegisterScheduleServiceServer(a.grpcServer, &grpcAdapter{transport: a.diContainer.ScheduleTransport()})
		slog.Info("gRPC server started", "address", a.lis.Addr().String())
		if err := a.grpcServer.Serve(a.lis); err != nil {
			slog.Error("gRPC serve error", "err", err)
		}
	}()

	return a.shutdown()
}

func (a *App) shutdown() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	defer signal.Stop(sigChan)

	<-sigChan

	a.db.Close()
	a.grpcServer.GracefulStop()
	err := a.nc.Drain()
	return err
}

func (a *App) initDeps() error {
	inits := []func() error{
		a.initNATS,
		a.initNetListener,
		a.initDB,
		a.initDI,
	}

	for _, f := range inits {
		if err := f(); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) initNATS() error {
	var err error
	a.nc, err = nats.Connect(a.cfg.NatsURL, nats.Name("scraper"))
	if err != nil {
		slog.Error("can't connect NATS", "url", a.cfg.NatsURL, "err", err)
		return err
	}
	return err
}

func (a *App) initNetListener() error {
	var err error
	a.lis, err = net.Listen("tcp", ":"+"9001")
	if err != nil {
		slog.Error("can't start net listener", "err", err)
		return err
	}
	return err
}

func (a *App) initDB() error {
	var err error
	a.db, err = database.New(a.cfg)
	if err != nil {
		slog.Error("can't connect to database")
		return err
	}
	if err := a.db.Ping(context.Background()); err != nil {
		slog.Error("Database ping error", "err", err)
		return err
	}
	return err
}

func (a *App) initDI() error {
	a.diContainer = NewDIContainer(a.nc, a.db.Pool)
	return nil
}
