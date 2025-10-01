package service

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type TelegramService interface {
	GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.Group, error)
	GetGroupSchedule(ctx context.Context, groupNum int) (*pb.Group, error)
}
