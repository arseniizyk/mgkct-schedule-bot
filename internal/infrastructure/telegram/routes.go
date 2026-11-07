package telegram

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/dictionary"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

// RegisterHandlers binds all Telegram command and event handlers to the telebot instance.
func RegisterHandlers(
	bot *tele.Bot,
	log *slog.Logger,
	cfg *config.Config,
	scheduleSvc *service.ScheduleService,
	teacherSvc *service.TeacherService,
	analyticsSvc *service.AnalyticsService,
	userRepo *repository.UserRepository,
	stateMgr *state.MemoryManager,
	subjectDict *dictionary.SubjectDictionary,
	mtprotoSender handlers.MTProtoSender,
) {
	adminMiddleware := handlers.AdminOnly(cfg.Bot.AdminIDs)

	bot.Handle("/start", handlers.Start(log, userRepo))
	bot.Handle("/help", handlers.Help(cfg.Bot.AdminIDs))
	bot.Handle("/app", handlers.App(cfg.Bot.WebAppURL))
	bot.Handle("/widget", handlers.Widget(log, scheduleSvc, teacherSvc, userRepo, cfg.Bot.WebAppURL, mtprotoSender))
	bot.Handle("/calls", handlers.Calls())
	bot.Handle("/cancel", handlers.Cancel(log, stateMgr))
	bot.Handle("/setgroup", handlers.SetGroup(log, scheduleSvc, userRepo, stateMgr))
	bot.Handle("/group", handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/pic", handlers.ScheduleImage(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle("/calendar", handlers.ScheduleCalendar(log, scheduleSvc, teacherSvc, userRepo, subjectDict))
	bot.Handle("/setteacher", handlers.SetTeacher(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/teacher", handlers.TeacherDay(log, teacherSvc, userRepo, stateMgr))
	bot.Handle("/subject", handlers.Subject(subjectDict, stateMgr))
	bot.Handle("/cabinet", handlers.Cabinet(log, analyticsSvc, stateMgr))
	bot.Handle("/endings", handlers.Endings(log, analyticsSvc))
	bot.Handle("/stats", handlers.Stats(log, analyticsSvc, userRepo, teacherSvc))
	bot.Handle("/archive", handlers.Archive(log, scheduleSvc, teacherSvc, userRepo, stateMgr))
	bot.Handle("/notify", handlers.Notify(log, userRepo))

	// Admin commands
	bot.Handle("/userstats", handlers.UserStats(log, userRepo), adminMiddleware)
	bot.Handle("/broadcast", handlers.Broadcast(log, stateMgr), adminMiddleware)
	bot.Handle("/botinfo", handlers.BotInfo(log, userRepo), adminMiddleware)

	bot.Handle(kbd.BtnDay, handlers.Day(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle(kbd.BtnWeek, handlers.Week(log, scheduleSvc, teacherSvc, userRepo))
	bot.Handle(kbd.BtnCalls, handlers.Calls())
	bot.Handle(kbd.BtnTeachers, handlers.Teachers(log, teacherSvc, userRepo, stateMgr))

	bot.Handle(tele.OnCallback, handlers.CallbacksHandler(log, scheduleSvc, teacherSvc, analyticsSvc, userRepo, stateMgr, bot, cfg.Bot.AdminIDs, subjectDict))
	bot.Handle(tele.OnText, handlers.StatesHandler(log, userRepo, stateMgr, teacherSvc, scheduleSvc, analyticsSvc, subjectDict, bot, cfg.Bot.AdminIDs))
	bot.Handle(tele.OnQuery, handlers.InlineQuery(log, scheduleSvc, teacherSvc, userRepo))
}

// BuildPoller creates a LongPoller or Webhook poller depending on configuration.
func BuildPoller(cfg *config.Config) (tele.Poller, http.Handler) {
	if cfg.Bot.WebhookURL == "" {
		return &tele.LongPoller{Timeout: 10 * time.Second}, nil
	}

	wh := &tele.Webhook{
		Listen: "",
		Endpoint: &tele.WebhookEndpoint{
			PublicURL: cfg.Bot.WebhookURL,
		},
		SecretToken: cfg.Bot.WebhookSecret,
		DropUpdates: false,
	}

	return wh, wh
}

// BuildBotPref constructs telebot.Settings based on configuration.
func BuildBotPref(cfg *config.Config) (tele.Settings, http.Handler) {
	poller, webhookHandler := BuildPoller(cfg)

	tgTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       300 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
	}

	botPref := tele.Settings{
		URL:    cfg.Bot.APIURL,
		Token:  cfg.Bot.Token,
		Poller: poller,
		Client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: tgTransport,
		},
	}

	return botPref, webhookHandler
}
