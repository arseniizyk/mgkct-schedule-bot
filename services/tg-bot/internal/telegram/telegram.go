package telegram

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
)

type UserUsecase interface {
	SaveUser(ctx context.Context, u *models.User) error
	SetUserGroup(ctx context.Context, chatID int64, groupID int) error
	GetGroupScheduleByID(ctx context.Context, groupID int) (*pb.GroupScheduleResponse, error)
	GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.GroupScheduleResponse, error)
}
