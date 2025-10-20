package app

import (
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	scheduleTransport "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
	"gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state"

	telegramRepository "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	telegramService "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
)

type diContainer struct {
	nc       *nats.Conn
	pool     *pgxpool.Pool
	grpcConn *grpc.ClientConn
	bot      *telebot.Bot

	scheduleTransport  scheduleTransport.Schedule
	telegramService    telegramService.Telegram
	telegramRepository telegramRepository.User
	telegramState      state.Manager
	telegramBotHandler *bot.Handler
}

func NewDIContainer(nc *nats.Conn, pool *pgxpool.Pool, grpcConn *grpc.ClientConn, bot *telebot.Bot) *diContainer {
	return &diContainer{
		nc:       nc,
		pool:     pool,
		grpcConn: grpcConn,
		bot:      bot,
	}
}

func (d *diContainer) ScheduleTransport() scheduleTransport.Schedule {
	if d.scheduleTransport == nil {
		d.scheduleTransport = scheduleTransport.New(d.nc, pb.NewScheduleServiceClient(d.grpcConn))
	}

	return d.scheduleTransport
}

func (d *diContainer) TelegramRepository() telegramRepository.User {
	if d.telegramRepository == nil {
		d.telegramRepository = telegramRepository.New(d.pool)
	}

	return d.telegramRepository
}

func (d *diContainer) TelegramService() telegramService.Telegram {
	if d.telegramService == nil {
		d.telegramService = telegramService.New(d.TelegramRepository(), d.ScheduleTransport())
	}

	return d.telegramService
}

func (d *diContainer) TelegramBotHandler() *bot.Handler {
	if d.telegramBotHandler == nil {
		d.telegramBotHandler = bot.NewHandler(d.TelegramRepository(), d.TelegramService(), d.TelegramState(), d.bot)
	}

	return d.telegramBotHandler
}

func (d *diContainer) TelegramState() state.Manager {
	if d.telegramState == nil {
		d.telegramState = state.NewMemory()
	}

	return d.telegramState
}
