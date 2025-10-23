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
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/service"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/transport"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
)

type ScheduleTransport interface {
	PublishScheduleUpdate(*pb.Group) error
	PublishWeekUpdates(date time.Time) error
	GetGroupSchedule(context.Context, *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error)
	GetGroupScheduleByWeek(context.Context, *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error)
	GetAvailableWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error)
}

type App struct {
	cfg               *config.Config
	lis               net.Listener
	db                *database.Database
	grpcServer        *grpc.Server
	nc                *nats.Conn
	scheduleSvc       transport.ScheduleService
	scheduleTransport ScheduleTransport
}

func New(cfg *config.Config) (*App, error) {
	a := &App{
		cfg: cfg,
		grpcServer: grpc.NewServer(
			grpc.UnaryInterceptor(loggingUnaryInterceptor),
		),
	}

	if err := a.initDeps(); err != nil {
		slog.Error("can't init dependencies", "err", err)
		return nil, err
	}

	scheduleRepo := repository.NewScheduleRepository(a.db.Pool)
	a.scheduleSvc = service.NewScheduleService(scheduleRepo)
	a.scheduleTransport = transport.NewScheduleTransport(a.scheduleSvc, a.nc)

	return a, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	go func() {
		updatesCh := a.scheduleSvc.CheckScheduleUpdates(time.Minute)
		for update := range updatesCh {
			if update.IsWeekUpdated {
				slog.Info("Week updated, publishing to NATS", "week", update.Group.Week.AsTime())
				if err := a.scheduleTransport.PublishWeekUpdates(update.Group.Week.AsTime()); err != nil {
					slog.Error("Failed publishing new week to NATS")
				}
				continue
			}

			if err := a.scheduleTransport.PublishScheduleUpdate(update.Group); err != nil {
				slog.Error("Failed publishing new schedule to NATS")
			}
		}
	}()

	go func() {
		pb.RegisterScheduleServiceServer(a.grpcServer, &grpcAdapter{transport: a.scheduleTransport})
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
