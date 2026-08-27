package errors

import "errors"

var (
	ErrUserNoGroup   = errors.New("user has no group")
	ErrUserNoTeacher = errors.New("user has no teacher")
	ErrUsersEmpty    = errors.New("users are empty")

	ErrWeekNotFound     = errors.New("week not found")
	ErrServiceInternal  = errors.New("service internal")
	ErrScheduleNotFound = errors.New("schedule not found")
	ErrGroupNotFound    = errors.New("group not found")
	ErrTeacherNotFound  = errors.New("teacher not found")
)
