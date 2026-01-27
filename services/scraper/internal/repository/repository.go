package repository

import "errors"

var (
	ErrWeekNotFound     = errors.New("week not found")
	ErrGroupNotFound    = errors.New("group not found")
	ErrScheduleNotFound = errors.New("schedule not found")
	ErrNoAvailableWeeks = errors.New("no available weeks")
)
