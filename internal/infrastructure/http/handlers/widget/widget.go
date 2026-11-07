package widget

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/widget"
)

type ScheduleService interface {
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error)
}

type TeacherService interface {
	GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error)
	ValidateTeacher(ctx context.Context, name string) (string, []string, bool)
}

type Handler struct {
	log         *slog.Logger
	scheduleSvc ScheduleService
	teacherSvc  TeacherService
}

func NewHandler(log *slog.Logger, scheduleSvc ScheduleService, teacherSvc TeacherService) *Handler {
	return &Handler{
		log:         log,
		scheduleSvc: scheduleSvc,
		teacherSvc:  teacherSvc,
	}
}

// Preview serves the Mini-HTML widget directly for browser / WebApp preview.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	const op = "http.widget.Preview"
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := r.URL.Query()
	groupStr := strings.TrimSpace(query.Get("group"))
	teacherQuery := strings.TrimSpace(query.Get("teacher"))

	var html string
	var err error

	switch {
	case groupStr != "":
		groupID, convErr := strconv.Atoi(groupStr)
		if convErr != nil || groupID <= 0 {
			http.Error(w, "invalid group parameter", http.StatusBadRequest)
			return
		}

		group, fetchErr := h.scheduleSvc.GetGroupLatestSchedule(ctx, int32(groupID))
		if fetchErr != nil {
			h.log.Warn("failed to get group schedule for widget preview", "operation", op, "group_id", groupID, "error", fetchErr)
			http.Error(w, fmt.Sprintf("failed to get schedule for group %d: %v", groupID, fetchErr), http.StatusNotFound)
			return
		}

		html, err = widget.BuildGroupWidget(group)

	case teacherQuery != "":
		matched, _, ok := h.teacherSvc.ValidateTeacher(ctx, teacherQuery)
		if !ok {
			matched = teacherQuery
		}

		teacher, fetchErr := h.teacherSvc.GetTeacherLatestSchedule(ctx, matched)
		if fetchErr != nil {
			h.log.Warn("failed to get teacher schedule for widget preview", "operation", op, "teacher", matched, "error", fetchErr)
			http.Error(w, fmt.Sprintf("failed to get schedule for teacher %s: %v", matched, fetchErr), http.StatusNotFound)
			return
		}

		html, err = widget.BuildTeacherWidget(teacher)

	default:
		// Demo placeholder if no group or teacher specified
		demoGroup := &entities.Group{
			ID:   81,
			Week: time.Now(),
			Days: []*entities.Day{
				{
					Name: "Понедельник",
					Subjects: []*entities.Subject{
						{
							Pairs: []*entities.Pair{
								{Name: "Основы алгоритмизации и программирования", Type: "ЛК", Class: "каб. 214", Group: "Иванов И.И."},
							},
						},
						{
							Pairs: []*entities.Pair{
								{Name: "1. Компьютерные сети", Type: "ЛБ", Class: "каб. 302", Group: "1 п/г"},
								{Name: "2. Базы данных", Type: "ПР", Class: "каб. 101", Group: "2 п/г"},
							},
						},
					},
				},
			},
		}
		html, err = widget.BuildGroupWidget(demoGroup)
	}

	if err != nil {
		h.log.Error("failed to generate widget HTML", "operation", op, "error", err)
		http.Error(w, "failed to generate widget HTML", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html)) //nolint:gosec // HTML is generated internally from validated schedule entities
}
