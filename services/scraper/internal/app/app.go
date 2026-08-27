package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	database "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/infrastructure/db"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/infrastructure/logger"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/infrastructure/parser"
	repository "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository/postgres"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/service"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/transport"
)

type WeekService interface {
	GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error)
}

type ScheduleService interface {
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error)
	CheckScheduleUpdates(ctx context.Context, interval time.Duration) <-chan *entities.UpdatedGroup
}

type ScheduleTransport interface {
	GetGroupSchedule(context.Context, *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error)
	GetGroupScheduleByWeek(context.Context, *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error)
	PublishScheduleUpdate(*pb.Group) error
}

type WeekTransport interface {
	GetAvailableWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error)
	PublishWeekUpdates(date time.Time) error
}

type TeacherScheduleService interface {
	GetTeacherLatestSchedule(ctx context.Context, name string) (*pb.Teacher, error)
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
	CheckTeacherScheduleUpdates(ctx context.Context, interval time.Duration) <-chan *entities.UpdatedTeacher
}

type TeacherScheduleTransport interface {
	GetTeacherSchedule(context.Context, *pb.TeacherScheduleRequest) (*pb.TeacherScheduleResponse, error)
	GetTeacherScheduleByWeek(context.Context, *pb.TeacherScheduleRequest) (*pb.TeacherScheduleResponse, error)
	GetAvailableTeacherWeeks(context.Context, *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error)
	GetTeacherNames(context.Context, *pb.Empty) (*pb.TeacherNamesResponse, error)
	PublishTeacherScheduleUpdate(*pb.Teacher) error
	PublishTeacherWeekUpdates(date time.Time) error
}

const checkScheduleInterval = 1 * time.Minute

type App struct {
	log *slog.Logger
	cfg *config.Config

	lis  net.Listener
	pool *pgxpool.Pool

	grpcServer *grpc.Server
	health     *health.Server
	nc         *nats.Conn

	wg sync.WaitGroup

	scheduleService   ScheduleService
	scheduleTransport ScheduleTransport

	weekService   WeekService
	weekTransport WeekTransport

	teacherScheduleService   TeacherScheduleService
	teacherScheduleTransport TeacherScheduleTransport
}

func New(log *slog.Logger, cfg *config.Config) (*App, error) {
	a := &App{
		log:        log,
		cfg:        cfg,
		grpcServer: grpc.NewServer(grpc.UnaryInterceptor(logger.LoggingUnaryInterceptor(log))),
		health:     health.NewServer(),
	}

	if err := a.initDeps(); err != nil {
		log.Error("can't init dependencies", "error", err)
		return nil, err
	}

	scheduleRepo := repository.NewScheduleRepository(a.pool)
	teacherScheduleRepo := repository.NewTeacherScheduleRepository(a.pool)

	studentParser := parser.New(a.log)
	teacherParser := parser.NewTeacherParser(a.log)

	a.scheduleService = service.NewScheduleService(a.log, scheduleRepo, studentParser)
	a.weekService = service.NewWeekService(a.log, scheduleRepo)
	a.teacherScheduleService = service.NewTeacherScheduleService(a.log, teacherScheduleRepo, teacherParser)

	a.scheduleTransport = transport.NewScheduleTransport(a.log, a.scheduleService, a.nc)
	a.weekTransport = transport.NewWeekTransport(a.log, a.weekService, a.nc)
	a.teacherScheduleTransport = transport.NewTeacherScheduleTransport(a.log, a.teacherScheduleService, a.nc)

	return a, nil
}

func (a *App) Run() error {
	log := a.log.With("operation", "app.App.Run")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a.wg.Go(func() {
		updatesCh := a.scheduleService.CheckScheduleUpdates(ctx, checkScheduleInterval)
		for update := range updatesCh {
			if update.IsWeekUpdated {
				log.Info("week updated, publishing to NATS", "week", update.Group.GetWeek().AsTime())
				if err := a.weekTransport.PublishWeekUpdates(update.Group.GetWeek().AsTime()); err != nil {
					log.Error("failed publishing new week to NATS", "err", err)
				} else {
					log.Info("updated Week Successfully published")
				}
				continue
			}

			if err := a.scheduleTransport.PublishScheduleUpdate(update.Group); err != nil {
				log.Error("failed publishing updated schedule to NATS", "err", err)
			} else {
				log.Info("updated schedule successfully published")
			}
		}
	})

	a.wg.Go(func() {
		teacherUpdatesCh := a.teacherScheduleService.CheckTeacherScheduleUpdates(ctx, checkScheduleInterval)
		for update := range teacherUpdatesCh {
			if update.IsWeekUpdated {
				log.Info("teacher week updated, publishing to NATS", "week", update.Teacher.GetWeek().AsTime())
				if err := a.teacherScheduleTransport.PublishTeacherWeekUpdates(update.Teacher.GetWeek().AsTime()); err != nil {
					log.Error("failed publishing new teacher week to NATS", "err", err)
				} else {
					log.Info("updated teacher week successfully published")
				}
				continue
			}

			if err := a.teacherScheduleTransport.PublishTeacherScheduleUpdate(update.Teacher); err != nil {
				log.Error("failed publishing updated teacher schedule to NATS", "err", err)
			} else {
				log.Info("updated teacher schedule successfully published")
			}
		}
	})

	go func() {
		pb.RegisterScheduleServiceServer(a.grpcServer, &grpcAdapter{
			scheduleTransport:        a.scheduleTransport,
			weekTransport:            a.weekTransport,
			teacherScheduleTransport: a.teacherScheduleTransport,
		})
		healthpb.RegisterHealthServer(a.grpcServer, a.health)
		a.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

		log.Info("gRPC server started", "address", a.lis.Addr().String())
		if err := a.grpcServer.Serve(a.lis); err != nil {
			log.Error("gRPC serve error", "err", err)
		}
	}()

	return a.shutdown(cancel)
}

func (a *App) shutdown(stopUpdates context.CancelFunc) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	defer signal.Stop(sigChan)

	<-sigChan

	a.health.Shutdown()
	a.grpcServer.GracefulStop()

	stopUpdates()

	a.wg.Wait()

	if err := a.nc.Drain(); err != nil {
		return fmt.Errorf("failed to drain from NATS: %w", err)
	}
	a.pool.Close()

	return nil
}

func (a *App) initDeps() error {
	inits := []func() error{
		a.initNATS,
		a.initNetListener,
		a.initDB,
	}

	for _, init := range inits {
		if err := init(); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) initNATS() error {
	var err error

	a.nc, err = nats.Connect(a.cfg.Nats.URL, nats.Name("scraper"))
	if err != nil {
		return fmt.Errorf("can't connect NATS: URL: %s, error: %w", a.cfg.Nats.URL, err)
	}

	return nil
}

func (a *App) initNetListener() error {
	var err error

	listenConfig := net.ListenConfig{}
	a.lis, err = listenConfig.Listen(context.Background(), "tcp", net.JoinHostPort("0.0.0.0", a.cfg.Scraper.GRPCPort))
	if err != nil {
		return fmt.Errorf("can't start net listener: Port: %s, error: %w", a.cfg.Scraper.GRPCPort, err)
	}

	return nil
}

func (a *App) initDB() error {
	ctx := context.Background()

	pool, err := database.Connect(ctx, &a.cfg.Scraper.DB)
	if err != nil {
		return fmt.Errorf("can't connect to database: %w", err)
	}
	a.pool = pool

	if err := a.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database error: %w", err)
	}

	return nil
}
