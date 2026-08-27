package teacherschedule

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/repository"
)

type TeacherScheduleTransport interface {
	GetTeacherSchedule(ctx context.Context, name string) (*pb.Teacher, error)
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error)
	GetAvailableTeacherWeeks(ctx context.Context, week *time.Time) (*pb.AvailableWeeksResponse, error)
	GetTeacherNames(ctx context.Context) ([]string, error)
}

type TeacherScheduleUsecase struct {
	log              *slog.Logger
	userRepository   repository.User
	teacherTransport TeacherScheduleTransport
}

func New(log *slog.Logger, userRepository repository.User, teacherTransport TeacherScheduleTransport) *TeacherScheduleUsecase {
	return &TeacherScheduleUsecase{
		log:              log,
		userRepository:   userRepository,
		teacherTransport: teacherTransport,
	}
}

func (s *TeacherScheduleUsecase) GetTeacherScheduleByChatID(ctx context.Context, chatID int64) (*pb.Teacher, error) {
	log := s.log.With("operation", "usecases.teacherschedule.TeacherScheduleUsecase.GetTeacherScheduleByChatID", "chat_id", chatID)

	teacherName, err := s.userRepository.GetTeacher(ctx, chatID)
	if err != nil {
		log.ErrorContext(ctx, "error getting user teacher from repository", "error", err)
		return nil, fmt.Errorf("get teacher: %w", err)
	}

	if teacherName == "" {
		return nil, domainerr.ErrUserNoTeacher
	}

	resp, err := s.teacherTransport.GetTeacherSchedule(ctx, teacherName)
	if err != nil {
		log.ErrorContext(ctx, "failed get teacher schedule:", "name", teacherName, "error", err)
		return nil, err
	}

	return resp, nil
}

func (s *TeacherScheduleUsecase) GetTeacherSchedule(ctx context.Context, name string) (*pb.Teacher, error) {
	log := s.log.With(
		"operation", "usecases.teacherschedule.TeacherScheduleUsecase.GetTeacherSchedule",
		"name", name,
	)

	matched, err := s.FindTeacherByName(ctx, name)
	if err != nil {
		log.ErrorContext(ctx, "failed find teacher by name", "error", err)
		return nil, err
	}

	if len(matched) == 0 {
		return nil, domainerr.ErrTeacherNotFound
	}

	resp, err := s.teacherTransport.GetTeacherSchedule(ctx, matched[0])
	if err != nil {
		log.ErrorContext(ctx, "failed get teacher schedule:", "error", err)
		return nil, err
	}

	return resp, nil
}

func (s *TeacherScheduleUsecase) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error) {
	log := s.log.With(
		"operation", "usecases.teacherschedule.TeacherScheduleUsecase.GetTeacherScheduleByWeek",
		"name", name,
		"week", week.String(),
	)

	resp, err := s.teacherTransport.GetTeacherScheduleByWeek(ctx, name, week)
	if err != nil {
		log.ErrorContext(ctx, "failed get teacher schedule by week", "error", err)
		return nil, err
	}

	return resp, nil
}

func (s *TeacherScheduleUsecase) GetAvailableTeacherWeeks(ctx context.Context, week *time.Time) (*pb.AvailableWeeksResponse, error) {
	log := s.log.With(
		"operation", "usecases.teacherschedule.TeacherScheduleUsecase.GetAvailableTeacherWeeks",
	)

	resp, err := s.teacherTransport.GetAvailableTeacherWeeks(ctx, week)
	if err != nil {
		log.ErrorContext(ctx, "failed get available teacher weeks", "error", err)
		return nil, err
	}

	return resp, nil
}

func (s *TeacherScheduleUsecase) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	log := s.log.With(
		"operation", "usecases.teacherschedule.TeacherScheduleUsecase.GetAllTeacherNames",
	)

	resp, err := s.teacherTransport.GetTeacherNames(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed get all teacher names", "error", err)
		return nil, err
	}

	return resp, nil
}

func (s *TeacherScheduleUsecase) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	log := s.log.With(
		"operation", "usecases.teacherschedule.TeacherScheduleUsecase.ValidateTeacher",
		"name", name,
	)

	candidates, err := s.FindTeacherByName(ctx, name)
	if err != nil {
		log.ErrorContext(ctx, "failed find teacher by name", "error", err)
		return "", nil, false
	}

	switch len(candidates) {
	case 0:
		return "", nil, false
	case 1:
		return candidates[0], nil, true
	default:
		return "", candidates, false
	}
}

func (s *TeacherScheduleUsecase) FindTeacherByName(ctx context.Context, name string) ([]string, error) {
	log := s.log.With(
		"operation", "usecases.teacherschedule.TeacherScheduleUsecase.FindTeacherByName",
		"name", name,
	)

	names, err := s.teacherTransport.GetTeacherNames(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed get teacher names", "error", err)
		return nil, err
	}

	lowerName := strings.ToLower(name)

	var exact, partial []string
	for _, n := range names {
		lower := strings.ToLower(n)
		switch {
		case lower == lowerName:
			exact = append(exact, n)
		case strings.Contains(lower, lowerName):
			partial = append(partial, n)
		}
	}

	return append(exact, partial...), nil
}
