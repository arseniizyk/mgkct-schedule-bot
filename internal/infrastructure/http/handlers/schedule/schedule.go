package schedule

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/calendar"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/dictionary"
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

func (h *Handler) Group(w http.ResponseWriter, r *http.Request) {
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
		sch, err := h.scheduleSvc.GetGroupScheduleByWeek(ctx, int32(id), parsedWeek)
		if err != nil {
			if errors.Is(err, domainerr.ErrGroupNotFound) || errors.Is(err, domainerr.ErrWeekNotFound) {
				http.Error(w, "group schedule not found", http.StatusNotFound)
				return
			}
			h.log.Error("failed getting group schedule by week", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		groupSchedule = sch
	} else {
		sch, err := h.scheduleSvc.GetGroupLatestSchedule(ctx, int32(id))
		if err != nil {
			if errors.Is(err, domainerr.ErrGroupNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
				http.Error(w, "group schedule not found", http.StatusNotFound)
				return
			}
			h.log.Error("failed getting latest group schedule", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		targetWeek = sch.Week
		groupSchedule = sch
	}

	weeksNav, _ := h.scheduleSvc.GetAvailableWeeks(ctx, targetWeek)

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"group": groupSchedule,
		"weeks": weeksNav,
	})
}

func (h *Handler) Teacher(w http.ResponseWriter, r *http.Request) {
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
		sch, err := h.teacherSvc.GetTeacherScheduleByWeek(ctx, name, parsedWeek)
		if err != nil {
			if errors.Is(err, domainerr.ErrTeacherNotFound) || errors.Is(err, domainerr.ErrWeekNotFound) {
				http.Error(w, "teacher schedule not found", http.StatusNotFound)
				return
			}
			h.log.Error("failed getting teacher schedule by week", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		teacherSchedule = sch
	} else {
		sch, err := h.teacherSvc.GetTeacherLatestSchedule(ctx, name)
		if err != nil {
			if errors.Is(err, domainerr.ErrTeacherNotFound) || errors.Is(err, domainerr.ErrScheduleNotFound) {
				http.Error(w, "teacher schedule not found", http.StatusNotFound)
				return
			}
			h.log.Error("failed getting latest teacher schedule", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		targetWeek = sch.Week
		teacherSchedule = sch
	}

	weeksNav, _ := h.teacherSvc.GetAvailableWeeks(ctx, name, targetWeek)

	httputil.WriteJSON(w, h.log, http.StatusOK, map[string]any{
		"teacher": teacherSchedule,
		"weeks":   weeksNav,
	})
}

func parseSubgroupParam(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	sg, err := strconv.Atoi(raw)
	if err != nil || sg < 0 || sg > 2 {
		return 0, errors.New("invalid subgroup")
	}
	return sg, nil
}

func parseCalendarWeek(weekStr string) (time.Time, error) {
	if weekStr == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", weekStr)
	if err != nil {
		parsed, err = time.Parse("02.01.2006", weekStr)
	}
	return parsed, err
}

func (h *Handler) CalendarICS(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	groupIDStr := q.Get("group")
	if groupIDStr == "" {
		groupIDStr = q.Get("id")
	}
	teacherName := q.Get("teacher")
	if teacherName == "" {
		teacherName = q.Get("name")
	}

	if groupIDStr == "" && teacherName == "" {
		http.Error(w, "group or teacher parameter is required", http.StatusBadRequest)
		return
	}

	targetWeek, err := parseCalendarWeek(q.Get("week"))
	if err != nil {
		http.Error(w, "invalid week format", http.StatusBadRequest)
		return
	}

	dict := dictionary.GetDefaultDictionary()

	if groupIDStr != "" {
		groupID, err := strconv.Atoi(groupIDStr)
		if err != nil || groupID <= 0 {
			http.Error(w, "invalid group id", http.StatusBadRequest)
			return
		}

		subgroup, err := parseSubgroupParam(q.Get("subgroup"))
		if err != nil {
			http.Error(w, "invalid subgroup", http.StatusBadRequest)
			return
		}

		h.serveGroupCalendar(w, r, groupID, targetWeek, subgroup, dict)
		return
	}

	h.serveTeacherCalendar(w, r, teacherName, targetWeek, dict)
}

func (h *Handler) serveGroupCalendar(w http.ResponseWriter, r *http.Request, groupID int, targetWeek time.Time, subgroup int, dict *dictionary.SubjectDictionary) {
	ctx := r.Context()
	var group *entities.Group
	var err error
	if !targetWeek.IsZero() {
		group, err = h.scheduleSvc.GetGroupScheduleByWeek(ctx, int32(groupID), targetWeek)
	} else {
		group, err = h.scheduleSvc.GetGroupLatestSchedule(ctx, int32(groupID))
	}
	if err != nil {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	icsData, err := calendar.GenerateGroupCalendar(group, subgroup, dict)
	if err != nil {
		h.log.Error("failed generating group calendar", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("schedule_group_%d_%s.ics", group.ID, group.Week.Format("20060102"))
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(icsData) //nolint:gosec // icsData is generated RFC 5545 calendar with text/calendar content type
}

func (h *Handler) serveTeacherCalendar(w http.ResponseWriter, r *http.Request, teacherName string, targetWeek time.Time, dict *dictionary.SubjectDictionary) {
	ctx := r.Context()
	var teacher *entities.Teacher
	var err error
	if !targetWeek.IsZero() {
		teacher, err = h.teacherSvc.GetTeacherScheduleByWeek(ctx, teacherName, targetWeek)
	} else {
		teacher, err = h.teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
	}
	if err != nil {
		http.Error(w, "teacher schedule not found", http.StatusNotFound)
		return
	}

	icsData, err := calendar.GenerateTeacherCalendar(teacher, dict)
	if err != nil {
		h.log.Error("failed generating teacher calendar", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("schedule_teacher_%s_%s.ics", strings.ReplaceAll(teacher.Name, " ", "_"), teacher.Week.Format("20060102"))
	encodedFilename := url.PathEscape(filename)
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"schedule_teacher.ics\"; filename*=UTF-8''%s", encodedFilename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(icsData) //nolint:gosec // icsData is generated RFC 5545 calendar with text/calendar content type
}
