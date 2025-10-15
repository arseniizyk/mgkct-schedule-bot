package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/model"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository"
)

func (s *service) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error) {
	sch, err := s.repo.GetByWeek(ctx, week)
	if err != nil {
		slog.Error("get by week error", "group_id", groupID, "week", week, "err", err)
		return nil, fmt.Errorf("get by week error: %w", err)
	}

	group, ok := sch.Groups[groupID]
	if !ok {
		slog.Warn("group not found", "group_id", groupID, "err", err)
		return nil, repository.ErrNotFound
	}

	return group, nil
}

func (s *service) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error) {
	if s.cache != nil {
		if group, ok := s.cache.Groups[groupID]; ok {
			return group, nil
		}
	}

	sch, err := s.repo.GetLatest(ctx)
	if err != nil {
		slog.Error("get by latest error", "group_id", groupID, "err", err)
		return nil, fmt.Errorf("get by week error: %w", err)
	}

	group, ok := sch.Groups[groupID]
	if !ok {
		slog.Warn("group not found", "group_id", groupID, "err", err)
		return nil, repository.ErrNotFound
	}

	return group, nil
}

func (s *service) GetAvailableWeeks(ctx context.Context, week time.Time) (*model.Weeks, error) {
	weeks, err := s.repo.GetWeeks(ctx, week)
	if err != nil {
		slog.Error("Service.GetAvailableWeeks.Repository.GetWeeks", "err", err)
		return nil, err
	}

	return weeks, nil
}
