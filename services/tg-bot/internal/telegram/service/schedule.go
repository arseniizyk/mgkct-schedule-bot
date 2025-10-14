package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	scheduleTransport "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
)

type Telegram interface {
	GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.Group, error)
	GetGroupSchedule(ctx context.Context, groupNum int) (*pb.Group, error)
	GetAvailableWeeks(ctx context.Context, week *time.Time) (models.Weeks, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error)
}

type service struct {
	userRepo          repository.TelegramUser
	scheduleTransport scheduleTransport.Schedule
}

func New(userRepo repository.TelegramUser, scheduleTransport scheduleTransport.Schedule) *service {
	return &service{
		scheduleTransport: scheduleTransport,
		userRepo:          userRepo,
	}
}

func (s *service) GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.Group, error) {
	groupNum, err := s.userRepo.GetUserGroup(ctx, chatID)
	if err != nil {
		if errors.Is(err, models.ErrUserNoGroup) {
			return nil, err
		}
		return nil, err
	}

	resp, err := s.scheduleTransport.GetGroupSchedule(ctx, groupNum)
	if err != nil {
		slog.Error("getgroupscheduleByChatID failed:", "chat_id", chatID, "group_id", groupNum, "err", err)
		return nil, err
	}

	return resp, nil
}

func (s *service) GetGroupSchedule(ctx context.Context, groupNum int) (*pb.Group, error) {
	resp, err := s.scheduleTransport.GetGroupSchedule(ctx, groupNum)
	if err != nil {
		slog.Error("getgroupschedule failed:", "group_id", groupNum, "err", err)
		return nil, err
	}

	return resp, nil
}

func (s *service) GetAvailableWeeks(ctx context.Context, week *time.Time) (models.Weeks, error) {
	resp, err := s.scheduleTransport.GetAvailableWeeks(ctx, week)
	if err != nil {
		slog.Error("getavailableweeks failed:", "week", week, "err", err)
		return models.Weeks{}, err
	}

	return resp, nil
}

func (s *service) GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error) {
	resp, err := s.scheduleTransport.GetGroupScheduleByWeek(ctx, groupID, week)
	if err != nil {
		slog.Error("Telegram.service.GetGroupScheduleByWeek", "group_id", groupID, "week", week.String(), "err", err)
		return nil, err
	}

	return resp, nil
}
