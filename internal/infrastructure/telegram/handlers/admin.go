package handlers

import (
	"context"
	"log/slog"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers/admin"
)

type (
	BotSender = admin.BotSender
	Limiter   = admin.Limiter
)

var (
	IsAdmin             = admin.IsAdmin
	AdminOnly           = admin.AdminOnly
	UserStats           = admin.UserStats
	BotInfo             = admin.BotInfo
	Broadcast           = admin.Broadcast
	SetBroadcastContext = admin.SetBroadcastContext
	SetBroadcastLimiter = admin.SetBroadcastLimiter
	SetBroadcastDraft   = admin.SetBroadcastDraft
	GetBroadcastDraft   = admin.GetBroadcastDraft
	ClearBroadcastDraft = admin.ClearBroadcastDraft
)

func handleWaitingBroadcast(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	sm StateManager,
) error {
	return admin.HandleWaitingBroadcast(ctx, log, c, sm)
}

func HandleBroadcastCallback(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	data string,
	userRepo UserRepo,
	bot BotSender,
	sm StateManager,
	adminIDs []int64,
) error {
	return admin.HandleBroadcastCallback(ctx, log, c, data, userRepo, bot, sm, adminIDs)
}
