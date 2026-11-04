package analytics

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/httputil"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type Handler struct {
	log          *slog.Logger
	analyticsSvc *service.AnalyticsService
}

func NewHandler(log *slog.Logger, analyticsSvc *service.AnalyticsService) *Handler {
	return &Handler{
		log:          log,
		analyticsSvc: analyticsSvc,
	}
}

func (h *Handler) Cabinet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cabinet := r.URL.Query().Get("number")
	if cabinet == "" {
		cabinet = r.URL.Query().Get("cabinet")
	}
	if cabinet == "" {
		http.Error(w, "cabinet number is required", http.StatusBadRequest)
		return
	}

	sched, candidates, err := h.analyticsSvc.GetCabinetSchedule(ctx, cabinet, time.Time{})
	if err != nil {
		h.log.Error("failed getting cabinet schedule", "error", err, "cabinet", cabinet)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if sched == nil {
		if len(candidates) > 0 {
			httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
				"candidates": candidates,
			})
			return
		}
		http.Error(w, "cabinet schedule not found", http.StatusNotFound)
		return
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"cabinet": sched,
	})
}

func (h *Handler) Endings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dayIdx := h.analyticsSvc.EffectiveEndingsDayIndex()
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

	endings, err := h.analyticsSvc.GetEndings(ctx, dayIdx, week)
	if err != nil {
		h.log.Error("failed getting endings", "error", err, "day", dayIdx)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"day_index": dayIdx,
		"endings":   endings,
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
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
		stats, total, err := h.analyticsSvc.GetGroupStats(ctx, int32(groupID), time.Time{})
		if err != nil {
			if errors.Is(err, domainerr.ErrGroupNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
				http.Error(w, "group schedule not found", http.StatusNotFound)
				return
			}
			h.log.Error("failed getting group stats", "error", err, "group", groupID)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
			"group_id": groupID,
			"stats":    stats,
			"total":    total,
		})
		return
	}

	stats, total, err := h.analyticsSvc.GetTeacherStats(ctx, teacher, time.Time{})
	if err != nil {
		if errors.Is(err, domainerr.ErrTeacherNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
			http.Error(w, "teacher schedule not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed getting teacher stats", "error", err, "teacher", teacher)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"teacher": teacher,
		"stats":   stats,
		"total":   total,
	})
}
