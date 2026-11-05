package user

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/auth"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/httputil"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type SetProfileRequest struct {
	GroupID     int    `json:"group_id"`
	Subgroup    int    `json:"subgroup"`
	TeacherName string `json:"teacher_name"`
}

type Handler struct {
	log         *slog.Logger
	cfg         *config.Config
	userRepo    *repository.UserRepository
	scheduleSvc *service.ScheduleService
	teacherSvc  *service.TeacherService
}

func NewHandler(
	log *slog.Logger,
	cfg *config.Config,
	userRepo *repository.UserRepository,
	scheduleSvc *service.ScheduleService,
	teacherSvc *service.TeacherService,
) *Handler {
	return &Handler{
		log:         log,
		cfg:         cfg,
		userRepo:    userRepo,
		scheduleSvc: scheduleSvc,
		teacherSvc:  teacherSvc,
	}
}

func (h *Handler) extractUser(r *http.Request) (*auth.TelegramUser, error) {
	authHeader := r.Header.Get("Authorization")
	if initData, ok := strings.CutPrefix(authHeader, "tma "); ok {
		u, err := auth.ValidateInitData(initData, h.cfg.Bot.Token, 15*time.Minute)
		if err == nil {
			return u, nil
		}
		h.log.Debug("initData validation failed", "error", err)
	}

	// Dev fallback
	if h.cfg.Env == config.EnvDev {
		if mockID := r.URL.Query().Get("mock_user_id"); mockID != "" {
			id, err := strconv.ParseInt(mockID, 10, 64)
			if err == nil {
				return &auth.TelegramUser{ID: id, Username: "dev_mock"}, nil
			}
		}
	}

	return nil, errors.New("unauthorized")
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	u, err := h.extractUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	groupID, subgroup, teacherName, err := h.userRepo.GetUserProfile(ctx, u.ID)
	if err != nil && !errors.Is(err, domainerr.ErrUserNoGroup) {
		h.log.Warn("failed getting user profile in webapp", "chat_id", u.ID, "error", err)
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"chat_id":      u.ID,
		"group_id":     groupID,
		"subgroup":     subgroup,
		"teacher_name": teacherName,
	})
}

func (h *Handler) SetProfile(w http.ResponseWriter, r *http.Request) {
	u, err := h.extractUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
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

	req.TeacherName = strings.TrimSpace(req.TeacherName)
	if req.GroupID <= 0 && req.TeacherName == "" {
		http.Error(w, "either group_id or teacher_name must be provided", http.StatusBadRequest)
		return
	}

	if req.Subgroup < 0 || req.Subgroup > 2 {
		http.Error(w, "invalid subgroup (expected 0, 1, or 2)", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if u.Username != "" {
		_ = h.userRepo.SaveUser(ctx, entities.NewUser(u.ID, u.Username))
	}

	if req.GroupID > 0 {
		if h.scheduleSvc != nil {
			exists, err := h.scheduleSvc.HasGroup(ctx, int32(req.GroupID))
			if err != nil {
				h.log.Error("failed checking group existence from webapp", "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if !exists {
				http.Error(w, "group not found", http.StatusBadRequest)
				return
			}
		}
		if err := h.userRepo.SetUserGroup(ctx, u.ID, req.GroupID, req.Subgroup); err != nil {
			h.log.Error("failed setting user group from webapp", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	} else if req.TeacherName != "" {
		if h.teacherSvc != nil {
			matched, _, ok := h.teacherSvc.ValidateTeacher(ctx, req.TeacherName)
			if !ok {
				http.Error(w, "teacher not found", http.StatusBadRequest)
				return
			}
			req.TeacherName = matched
		}
		if err := h.userRepo.SetTeacher(ctx, u.ID, req.TeacherName); err != nil {
			h.log.Error("failed setting teacher from webapp", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"success": true,
	})
}
