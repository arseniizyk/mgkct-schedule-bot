package archive

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/httputil"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type Handler struct {
	log         *slog.Logger
	scheduleSvc *service.ScheduleService
	teacherSvc  *service.TeacherService
}

func NewHandler(log *slog.Logger, scheduleSvc *service.ScheduleService, teacherSvc *service.TeacherService) *Handler {
	return &Handler{
		log:         log,
		scheduleSvc: scheduleSvc,
		teacherSvc:  teacherSvc,
	}
}

func (h *Handler) Weeks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	teacher := r.URL.Query().Get("teacher")

	var weeks []time.Time
	var err error

	if teacher != "" {
		weeks, err = h.teacherSvc.GetAllWeeks(ctx, teacher)
	} else {
		weeks, err = h.scheduleSvc.GetAllWeeks(ctx)
	}

	if err != nil {
		h.log.Error("failed getting archive weeks", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	weekStrs := make([]string, 0, len(weeks))
	for _, w := range weeks {
		weekStrs = append(weekStrs, w.Format("2006-01-02"))
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"weeks": weekStrs,
	})
}
