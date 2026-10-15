package service

import (
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

type EventType string

const (
	EventNewWeek                EventType = "new_week"
	EventGroupScheduleUpdated   EventType = "group_schedule_updated"
	EventTeacherScheduleUpdated EventType = "teacher_schedule_updated"
	EventScraperError           EventType = "scraper_error"
	EventScraperRecovered       EventType = "scraper_recovered"
)

type Event struct {
	Type             EventType
	Week             time.Time
	Group            *entities.Group
	Teacher          *entities.Teacher
	Diff             *entities.ScheduleDiff
	Err              error
	ConsecutiveFails int
}
