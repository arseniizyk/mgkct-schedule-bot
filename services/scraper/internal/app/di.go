package app

import (
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/service"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/transport"

	scheduleRepository "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository/schedule"
	scheduleService "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/service/schedule"
	scheduleTransport "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/transport/schedule"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type diContainer struct {
	nc                 *nats.Conn
	pool               *pgxpool.Pool
	scheduleTransport  transport.ScheduleTransport
	scheduleService    service.ScheduleService
	scheduleRepository repository.ScheduleRepository
}

func NewDIContainer(nc *nats.Conn, pool *pgxpool.Pool) *diContainer {
	return &diContainer{
		nc:   nc,
		pool: pool,
	}
}

func (d *diContainer) ScheduleTransport() transport.ScheduleTransport {
	if d.scheduleTransport == nil {
		d.scheduleTransport = scheduleTransport.New(d.ScheduleService(), d.nc)
	}

	return d.scheduleTransport
}

func (d *diContainer) ScheduleService() service.ScheduleService {
	if d.scheduleService == nil {
		d.scheduleService = scheduleService.NewScheduleService(d.ScheduleRepository())
	}

	return d.scheduleService
}

func (d *diContainer) ScheduleRepository() repository.ScheduleRepository {
	if d.scheduleRepository == nil {
		d.scheduleRepository = scheduleRepository.New(d.pool)
	}

	return d.scheduleRepository
}
