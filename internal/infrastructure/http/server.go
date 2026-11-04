package httpserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/analytics"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/archive"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/directory"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/health"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/schedule"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/static"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/handlers/user"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
	"github.com/arseniizyk/mgkct-schedule-bot/web"
)

type CallItem = directory.CallItem

type Server struct {
	log          *slog.Logger
	cfg          *config.Config
	database     *sql.DB
	scheduleSvc  *service.ScheduleService
	teacherSvc   *service.TeacherService
	analyticsSvc *service.AnalyticsService
	userRepo     *repository.UserRepository
	httpSrv      *http.Server
	mux          *http.ServeMux
}

func New(
	log *slog.Logger,
	cfg *config.Config,
	database *sql.DB,
	scheduleSvc *service.ScheduleService,
	teacherSvc *service.TeacherService,
	analyticsSvc *service.AnalyticsService,
	userRepo *repository.UserRepository,
) *Server {
	s := &Server{
		log:          log,
		cfg:          cfg,
		database:     database,
		scheduleSvc:  scheduleSvc,
		teacherSvc:   teacherSvc,
		analyticsSvc: analyticsSvc,
		userRepo:     userRepo,
	}

	mux := http.NewServeMux()
	s.mux = mux

	// Subpackage handlers
	healthH := health.NewHandler(database)
	schedH := schedule.NewHandler(log, scheduleSvc, teacherSvc)
	dirH := directory.NewHandler(log, scheduleSvc, teacherSvc, analyticsSvc)
	analyticsH := analytics.NewHandler(log, analyticsSvc)
	archiveH := archive.NewHandler(log, scheduleSvc, teacherSvc)
	userH := user.NewHandler(log, cfg, userRepo, scheduleSvc, teacherSvc)

	// Healthcheck
	mux.HandleFunc("GET /healthz", healthH)

	// API v1
	mux.HandleFunc("GET /api/v1/schedule/group", schedH.Group)
	mux.HandleFunc("GET /api/v1/schedule/teacher", schedH.Teacher)
	mux.HandleFunc("GET /api/v1/schedule/calendar.ics", schedH.CalendarICS)
	mux.HandleFunc("GET /api/v1/groups", dirH.Groups)
	mux.HandleFunc("GET /api/v1/teachers", dirH.Teachers)
	mux.HandleFunc("GET /api/v1/subjects", dirH.Subjects)
	mux.HandleFunc("GET /api/v1/cabinets", dirH.Cabinets)
	mux.HandleFunc("GET /api/v1/cabinet", analyticsH.Cabinet)
	mux.HandleFunc("GET /api/v1/analytics/endings", analyticsH.Endings)
	mux.HandleFunc("GET /api/v1/analytics/stats", analyticsH.Stats)
	mux.HandleFunc("GET /api/v1/archive/weeks", archiveH.Weeks)
	mux.HandleFunc("GET /api/v1/calls", dirH.Calls)
	mux.HandleFunc("GET /api/v1/user/profile", userH.GetProfile)
	mux.HandleFunc("POST /api/v1/user/profile", userH.SetProfile)

	// Static SPA WebApp handler
	staticHandler := static.SPAHandler(web.Assets)
	mux.Handle("/", staticHandler)

	handler := s.corsMiddleware(mux)

	addr := net.JoinHostPort("0.0.0.0", cfg.Bot.HealthPort)
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s
}

// RegisterWebhook регистрирует обработчик вебхука Telegram в HTTP-роутере.
// Должен вызываться до старта HTTP-сервера.
func (s *Server) RegisterWebhook(path string, handler http.Handler) {
	s.mux.Handle("POST "+path, handler)
}

func (s *Server) Start() error {
	s.log.Info("Starting HTTP server", "addr", s.httpSrv.Addr)
	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server failed: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down HTTP server...")
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
