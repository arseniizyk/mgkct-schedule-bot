package directory

import (
	"log/slog"
	"net/http"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/http/httputil"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type CallItem struct {
	Number        int    `json:"number"`
	Time          string `json:"time"`
	Start         string `json:"start"`
	End           string `json:"end"`
	FirstHalf     string `json:"first_half"`
	SecondHalf    string `json:"second_half"`
	BreakDuration string `json:"break_duration"`
}

type Handler struct {
	log          *slog.Logger
	scheduleSvc  *service.ScheduleService
	teacherSvc   *service.TeacherService
	analyticsSvc *service.AnalyticsService
}

func NewHandler(
	log *slog.Logger,
	scheduleSvc *service.ScheduleService,
	teacherSvc *service.TeacherService,
	analyticsSvc *service.AnalyticsService,
) *Handler {
	return &Handler{
		log:          log,
		scheduleSvc:  scheduleSvc,
		teacherSvc:   teacherSvc,
		analyticsSvc: analyticsSvc,
	}
}

func (h *Handler) Groups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ids, err := h.scheduleSvc.GetAllGroupIDs(ctx)
	if err != nil {
		h.log.Error("failed getting all group ids", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"groups": ids,
	})
}

func (h *Handler) Teachers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	names, err := h.teacherSvc.GetAllTeacherNames(ctx)
	if err != nil {
		h.log.Error("failed getting teacher names", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	faculty, _ := h.teacherSvc.GetAllFaculty(ctx)

	resp := map[string]any{
		"teachers": names,
	}
	if len(faculty) > 0 {
		resp["directory"] = faculty
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, resp)
}

func (h *Handler) Subjects(w http.ResponseWriter, _ *http.Request) {
	dict := h.teacherSvc.GetSubjectDictionary()
	if dict == nil {
		httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{"subjects": map[string]string{}})
		return
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"subjects": dict.All(),
	})
}

func (h *Handler) Cabinets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cabinets, err := h.analyticsSvc.GetAllCabinets(ctx)
	if err != nil {
		h.log.Error("failed getting cabinets list", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"cabinets": cabinets,
	})
}

func (h *Handler) Calls(w http.ResponseWriter, _ *http.Request) {
	calls := []CallItem{
		{Number: 1, Time: "9:00 - 10:40", Start: "09:00", End: "10:40", FirstHalf: "9:00 - 9:45", SecondHalf: "9:55 - 10:40", BreakDuration: "10 мин перерыв"},
		{Number: 2, Time: "10:50 - 12:40", Start: "10:50", End: "12:40", FirstHalf: "10:50 - 11:35", SecondHalf: "11:55 - 12:40", BreakDuration: "20 мин обед"},
		{Number: 3, Time: "13:00 - 14:40", Start: "13:00", End: "14:40", FirstHalf: "13:00 - 13:45", SecondHalf: "13:55 - 14:40", BreakDuration: "10 мин перерыв"},
		{Number: 4, Time: "14:50 - 16:30", Start: "14:50", End: "16:30", FirstHalf: "14:50 - 15:35", SecondHalf: "15:45 - 16:30", BreakDuration: "10 мин перерыв"},
		{Number: 5, Time: "16:40 - 18:20", Start: "16:40", End: "18:20", FirstHalf: "16:40 - 17:25", SecondHalf: "17:35 - 18:20", BreakDuration: "10 мин перерыв"},
		{Number: 6, Time: "18:30 - 20:10", Start: "18:30", End: "20:10", FirstHalf: "18:30 - 19:15", SecondHalf: "19:25 - 20:10", BreakDuration: "Конец занятий"},
	}

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"calls": calls,
	})
}
