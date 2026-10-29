package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/auth"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
	"github.com/arseniizyk/mgkct-schedule-bot/web"
)

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

	// Healthcheck
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	// API v1
	mux.HandleFunc("GET /api/v1/schedule/group", s.handleGroupSchedule)
	mux.HandleFunc("GET /api/v1/schedule/teacher", s.handleTeacherSchedule)
	mux.HandleFunc("GET /api/v1/groups", s.handleGroupsList)
	mux.HandleFunc("GET /api/v1/teachers", s.handleTeachersList)
	mux.HandleFunc("GET /api/v1/subjects", s.handleSubjectsList)
	mux.HandleFunc("GET /api/v1/cabinets", s.handleCabinetsList)
	mux.HandleFunc("GET /api/v1/cabinet", s.handleCabinetSchedule)
	mux.HandleFunc("GET /api/v1/analytics/endings", s.handleEndings)
	mux.HandleFunc("GET /api/v1/analytics/stats", s.handleStats)
	mux.HandleFunc("GET /api/v1/archive/weeks", s.handleArchiveWeeks)
	mux.HandleFunc("GET /api/v1/calls", s.handleCalls)
	mux.HandleFunc("GET /api/v1/user/profile", s.handleGetUserProfile)
	mux.HandleFunc("POST /api/v1/user/profile", s.handleSetUserProfile)

	// Static SPA WebApp handler
	staticHandler := s.spaHandler(web.Assets)
	mux.Handle("/", staticHandler)

	handler := s.corsMiddleware(mux)

	addr := net.JoinHostPort("0.0.0.0", cfg.Bot.HealthPort)
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
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
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
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

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.database.PingContext(pingCtx); err != nil {
		http.Error(w, "database unhealthy: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) handleGroupSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "group id is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}

	weekStr := r.URL.Query().Get("week")
	var groupSchedule any
	var targetWeek time.Time

	if weekStr != "" {
		parsedWeek, err := time.Parse("2006-01-02", weekStr)
		if err != nil {
			http.Error(w, "invalid week format (YYYY-MM-DD expected)", http.StatusBadRequest)
			return
		}
		targetWeek = parsedWeek
		sch, err := s.scheduleSvc.GetGroupScheduleByWeek(ctx, int32(id), parsedWeek)
		if err != nil {
			if errors.Is(err, domainerr.ErrGroupNotFound) || errors.Is(err, domainerr.ErrWeekNotFound) {
				http.Error(w, "group schedule not found", http.StatusNotFound)
				return
			}
			s.log.Error("failed getting group schedule by week", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		groupSchedule = sch
	} else {
		sch, err := s.scheduleSvc.GetGroupLatestSchedule(ctx, int32(id))
		if err != nil {
			if errors.Is(err, domainerr.ErrGroupNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
				http.Error(w, "group schedule not found", http.StatusNotFound)
				return
			}
			s.log.Error("failed getting latest group schedule", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		targetWeek = sch.Week
		groupSchedule = sch
	}

	weeksNav, _ := s.scheduleSvc.GetAvailableWeeks(ctx, targetWeek)

	s.writeJSON(w, http.StatusOK, map[string]any{
		"group": groupSchedule,
		"weeks": weeksNav,
	})
}

func (s *Server) handleTeacherSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "teacher name is required", http.StatusBadRequest)
		return
	}

	weekStr := r.URL.Query().Get("week")
	var teacherSchedule any
	var targetWeek time.Time

	if weekStr != "" {
		parsedWeek, err := time.Parse("2006-01-02", weekStr)
		if err != nil {
			http.Error(w, "invalid week format (YYYY-MM-DD expected)", http.StatusBadRequest)
			return
		}
		targetWeek = parsedWeek
		sch, err := s.teacherSvc.GetTeacherScheduleByWeek(ctx, name, parsedWeek)
		if err != nil {
			if errors.Is(err, domainerr.ErrTeacherNotFound) || errors.Is(err, domainerr.ErrWeekNotFound) {
				http.Error(w, "teacher schedule not found", http.StatusNotFound)
				return
			}
			s.log.Error("failed getting teacher schedule by week", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		teacherSchedule = sch
	} else {
		sch, err := s.teacherSvc.GetTeacherLatestSchedule(ctx, name)
		if err != nil {
			if errors.Is(err, domainerr.ErrTeacherNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
				http.Error(w, "teacher schedule not found", http.StatusNotFound)
				return
			}
			s.log.Error("failed getting latest teacher schedule", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		targetWeek = sch.Week
		teacherSchedule = sch
	}

	weeksNav, _ := s.teacherSvc.GetAvailableWeeks(ctx, name, targetWeek)

	s.writeJSON(w, http.StatusOK, map[string]any{
		"teacher": teacherSchedule,
		"weeks":   weeksNav,
	})
}

func (s *Server) handleGroupsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ids, err := s.scheduleSvc.GetAllGroupIDs(ctx)
	if err != nil {
		s.log.Error("failed getting all group ids", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"groups": ids,
	})
}

func (s *Server) handleTeachersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	names, err := s.teacherSvc.GetAllTeacherNames(ctx)
	if err != nil {
		s.log.Error("failed getting teacher names", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	faculty, _ := s.teacherSvc.GetAllFaculty(ctx)

	resp := map[string]any{
		"teachers": names,
	}
	if len(faculty) > 0 {
		resp["directory"] = faculty
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSubjectsList(w http.ResponseWriter, _ *http.Request) {
	dict := s.teacherSvc.GetSubjectDictionary()
	if dict == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"subjects": map[string]string{}})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"subjects": dict.All(),
	})
}

func (s *Server) handleCabinetsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cabinets, err := s.analyticsSvc.GetAllCabinets(ctx)
	if err != nil {
		s.log.Error("failed getting cabinets list", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"cabinets": cabinets,
	})
}

func (s *Server) handleCabinetSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cabinet := r.URL.Query().Get("number")
	if cabinet == "" {
		cabinet = r.URL.Query().Get("cabinet")
	}
	if cabinet == "" {
		http.Error(w, "cabinet number is required", http.StatusBadRequest)
		return
	}

	sched, candidates, err := s.analyticsSvc.GetCabinetSchedule(ctx, cabinet, time.Time{})
	if err != nil {
		s.log.Error("failed getting cabinet schedule", "error", err, "cabinet", cabinet)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if sched == nil {
		if len(candidates) > 0 {
			s.writeJSON(w, http.StatusOK, map[string]any{
				"candidates": candidates,
			})
			return
		}
		http.Error(w, "cabinet schedule not found", http.StatusNotFound)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"cabinet": sched,
	})
}

func (s *Server) handleEndings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dayIdx := s.analyticsSvc.EffectiveEndingsDayIndex()
	if dayStr := r.URL.Query().Get("day"); dayStr != "" {
		parsedDay, err := strconv.Atoi(dayStr)
		if err != nil || parsedDay < 0 || parsedDay > 6 {
			http.Error(w, "invalid day parameter (0-6 expected)", http.StatusBadRequest)
			return
		}
		dayIdx = parsedDay
	}

	var week time.Time
	if weekStr := r.URL.Query().Get("week"); weekStr != "" {
		parsedWeek, err := time.Parse("2006-01-02", weekStr)
		if err != nil {
			http.Error(w, "invalid week parameter (YYYY-MM-DD expected)", http.StatusBadRequest)
			return
		}
		week = parsedWeek
	}

	endings, err := s.analyticsSvc.GetEndings(ctx, dayIdx, week)
	if err != nil {
		s.log.Error("failed getting endings", "error", err, "day", dayIdx)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"day_index": dayIdx,
		"endings":   endings,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupStr := r.URL.Query().Get("group")
	teacher := r.URL.Query().Get("teacher")

	if groupStr == "" && teacher == "" {
		http.Error(w, "either group or teacher parameter is required", http.StatusBadRequest)
		return
	}

	if groupStr != "" {
		groupID, err := strconv.Atoi(groupStr)
		if err != nil {
			http.Error(w, "invalid group id", http.StatusBadRequest)
			return
		}
		stats, total, err := s.analyticsSvc.GetGroupStats(ctx, int32(groupID), time.Time{})
		if err != nil {
			if errors.Is(err, domainerr.ErrGroupNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
				http.Error(w, "group schedule not found", http.StatusNotFound)
				return
			}
			s.log.Error("failed getting group stats", "error", err, "group", groupID)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{
			"group_id": groupID,
			"stats":    stats,
			"total":    total,
		})
		return
	}

	stats, total, err := s.analyticsSvc.GetTeacherStats(ctx, teacher, time.Time{})
	if err != nil {
		if errors.Is(err, domainerr.ErrTeacherNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
			http.Error(w, "teacher schedule not found", http.StatusNotFound)
			return
		}
		s.log.Error("failed getting teacher stats", "error", err, "teacher", teacher)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"teacher": teacher,
		"stats":   stats,
		"total":   total,
	})
}

func (s *Server) handleArchiveWeeks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	teacher := r.URL.Query().Get("teacher")

	var weeks []time.Time
	var err error

	if teacher != "" {
		weeks, err = s.teacherSvc.GetAllWeeks(ctx, teacher)
	} else {
		weeks, err = s.scheduleSvc.GetAllWeeks(ctx)
	}

	if err != nil {
		s.log.Error("failed getting archive weeks", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	weekStrs := make([]string, 0, len(weeks))
	for _, w := range weeks {
		weekStrs = append(weekStrs, w.Format("2006-01-02"))
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"weeks": weekStrs,
	})
}

type CallItem struct {
	Number        int    `json:"number"`
	Time          string `json:"time"`
	Start         string `json:"start"`
	End           string `json:"end"`
	FirstHalf     string `json:"first_half"`
	SecondHalf    string `json:"second_half"`
	BreakDuration string `json:"break_duration"`
}

func (s *Server) handleCalls(w http.ResponseWriter, _ *http.Request) {
	calls := []CallItem{
		{Number: 1, Time: "9:00 - 10:40", Start: "09:00", End: "10:40", FirstHalf: "9:00 - 9:45", SecondHalf: "9:55 - 10:40", BreakDuration: "10 мин перерыв"},
		{Number: 2, Time: "10:50 - 12:40", Start: "10:50", End: "12:40", FirstHalf: "10:50 - 11:35", SecondHalf: "11:55 - 12:40", BreakDuration: "20 мин обед"},
		{Number: 3, Time: "13:00 - 14:40", Start: "13:00", End: "14:40", FirstHalf: "13:00 - 13:45", SecondHalf: "13:55 - 14:40", BreakDuration: "10 мин перерыв"},
		{Number: 4, Time: "14:50 - 16:30", Start: "14:50", End: "16:30", FirstHalf: "14:50 - 15:35", SecondHalf: "15:45 - 16:30", BreakDuration: "10 мин перерыв"},
		{Number: 5, Time: "16:40 - 18:20", Start: "16:40", End: "18:20", FirstHalf: "16:40 - 17:25", SecondHalf: "17:35 - 18:20", BreakDuration: "10 мин перерыв"},
		{Number: 6, Time: "18:30 - 20:10", Start: "18:30", End: "20:10", FirstHalf: "18:30 - 19:15", SecondHalf: "19:25 - 20:10", BreakDuration: "Конец занятий"},
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"calls": calls,
	})
}

func (s *Server) extractUser(r *http.Request) (*auth.TelegramUser, error) {
	authHeader := r.Header.Get("Authorization")
	if initData, ok := strings.CutPrefix(authHeader, "tma "); ok {
		user, err := auth.ValidateInitData(initData, s.cfg.Bot.Token, 24*time.Hour)
		if err == nil {
			return user, nil
		}
		s.log.Debug("initData validation failed", "error", err)
	}

	// Dev fallback
	if s.cfg.Env == config.EnvDev {
		if mockID := r.URL.Query().Get("mock_user_id"); mockID != "" {
			id, err := strconv.ParseInt(mockID, 10, 64)
			if err == nil {
				return &auth.TelegramUser{ID: id, Username: "dev_mock"}, nil
			}
		}
	}

	return nil, errors.New("unauthorized")
}

func (s *Server) handleGetUserProfile(w http.ResponseWriter, r *http.Request) {
	user, err := s.extractUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	groupID, _ := s.userRepo.GroupByChatID(ctx, user.ID)
	subgroup, _ := s.userRepo.SubgroupByChatID(ctx, user.ID)
	teacherName, _ := s.userRepo.GetTeacher(ctx, user.ID)

	s.writeJSON(w, http.StatusOK, map[string]any{
		"chat_id":      user.ID,
		"group_id":     groupID,
		"subgroup":     subgroup,
		"teacher_name": teacherName,
	})
}

type SetProfileRequest struct {
	GroupID     int    `json:"group_id"`
	Subgroup    int    `json:"subgroup"`
	TeacherName string `json:"teacher_name"`
}

func (s *Server) handleSetUserProfile(w http.ResponseWriter, r *http.Request) {
	user, err := s.extractUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed reading body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req SetProfileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if req.GroupID > 0 {
		if err := s.userRepo.SetUserGroup(ctx, user.ID, req.GroupID, req.Subgroup); err != nil {
			s.log.Error("failed setting user group from webapp", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	} else if req.TeacherName != "" {
		if err := s.userRepo.SetTeacher(ctx, user.ID, req.TeacherName); err != nil {
			s.log.Error("failed setting teacher from webapp", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (s *Server) spaHandler(files fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(files))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Проверяем, существует ли файл в встроенной ФС
		f, err := files.Open(path)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Если файл не найден (например, глубокий маршрут SPA), отдаём index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.log.Error("failed encoding json response", "error", err)
	}
}
