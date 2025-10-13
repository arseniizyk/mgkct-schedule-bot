package app

import (
	scheduleRepository "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository"
	scheduleService "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/service"
	scheduleTransport "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/transport"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type diContainer struct {
	nc                 *nats.Conn
	pool               *pgxpool.Pool
	scheduleTransport  scheduleTransport.Schedule
	scheduleService    scheduleService.Schedule
	scheduleRepository scheduleRepository.Schedule
}

func NewDIContainer(nc *nats.Conn, pool *pgxpool.Pool) *diContainer {
	return &diContainer{
		nc:   nc,
		pool: pool,
	}
}

func (d *diContainer) ScheduleTransport() scheduleTransport.Schedule {
	if d.scheduleTransport == nil {
		d.scheduleTransport = scheduleTransport.New(d.ScheduleService(), d.nc)
	}

	return d.scheduleTransport
}

func (d *diContainer) ScheduleService() scheduleService.Schedule {
	if d.scheduleService == nil {
		d.scheduleService = scheduleService.NewScheduleService(d.ScheduleRepository())
	}

	return d.scheduleService
}

func (d *diContainer) ScheduleRepository() scheduleRepository.Schedule {
	if d.scheduleRepository == nil {
		d.scheduleRepository = scheduleRepository.New(d.pool)
	}

	return d.scheduleRepository
}
