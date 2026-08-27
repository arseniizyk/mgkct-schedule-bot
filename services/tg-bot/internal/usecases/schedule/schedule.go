package schedule

import (
	"context"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/repository"
)

type ScheduleTransport interface {
	GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error)
	GetGroupSchedule(ctx context.Context, groupID int) (*pb.Group, error)
}

type ScheduleUsecase struct {
	log               *slog.Logger
	userRepository    repository.User
	scheduleTransport ScheduleTransport
}

func New(log *slog.Logger, userRepository repository.User, scheduleTransport ScheduleTransport) *ScheduleUsecase {
	return &ScheduleUsecase{
		log:               log,
		userRepository:    userRepository,
		scheduleTransport: scheduleTransport,
	}
}

func (s *ScheduleUsecase) GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.Group, error) {
	log := s.log.With("operation", "service.telegram.TelegramService.GetGroupScheduleByChatID", "chat_id", chatID)

	groupID, err := s.userRepository.GroupByChatID(ctx, chatID)
	if err != nil {
		log.ErrorContext(ctx, "error getting user group from repository", "error", err)
		return nil, err
	}

	resp, err := s.scheduleTransport.GetGroupSchedule(ctx, groupID)
	if err != nil {
		log.ErrorContext(ctx, "failed get group schedule:", "group_id", groupID, "error", err)
		return nil, err
	}

	return resp, nil
}

func (s *ScheduleUsecase) GetGroupSchedule(ctx context.Context, groupID int) (*pb.Group, error) {
	log := s.log.With(
		"operation", "service.telegram.TelegramService.GetGroupSchedule",
		"group_id", groupID,
	)

	resp, err := s.scheduleTransport.GetGroupSchedule(ctx, groupID)
	if err != nil {
		log.ErrorContext(ctx, "failed get group schedule:", "error", err)
		return nil, err
	}

	return resp, nil
}

func (s *ScheduleUsecase) GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error) {
	log := s.log.With(
		"operation", "service.telegram.TelegramService.GetGroupScheduleByWeek",
		"group_id", groupID,
		"week", week.String(),
	)

	resp, err := s.scheduleTransport.GetGroupScheduleByWeek(ctx, groupID, week)
	if err != nil {
		log.ErrorContext(ctx, "failed group schedule by week", "error", err)
		return nil, err
	}

	return resp, nil
}
