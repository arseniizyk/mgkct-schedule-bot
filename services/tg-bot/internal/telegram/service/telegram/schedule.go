package telegram

import (
	"context"
	"errors"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	schedule "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
)

type service struct {
	userRepo          repository.TelegramUserRepository
	scheduleTransport schedule.ScheduleTransport
}

func New(userRepo repository.TelegramUserRepository, scheduleTransport schedule.ScheduleTransport) *service {
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
