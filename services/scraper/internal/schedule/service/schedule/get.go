package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

func (s *service) GetFullLatestSchedule(ctx context.Context) (*pb.Schedule, error) {
	if s.cache != nil {
		return s.cache, nil
	}

	sch, err := s.repo.GetLatest(ctx)
	if err != nil {
		slog.Error("get latest error; ", "err", err)
		return nil, fmt.Errorf("get latest schedule: %w", err)
	}

	s.cache = sch
	return sch, nil
}

func (s *service) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error) {
	if s.cache != nil {
		if group, ok := s.cache.Groups[groupID]; ok && !group.Week.AsTime().After(week) {
			return group, nil
		}
	}

	sch, err := s.repo.GetByWeek(ctx, week)
	if err != nil {
		slog.Error("get by week error", "group_id", groupID, "week", week, "err", err)
		return nil, fmt.Errorf("get by week error")
	}

	group, ok := sch.Groups[groupID]
	if !ok {
		slog.Warn("group not found", "group_id", groupID, "err", err)
		return nil, fmt.Errorf("group not found: %w", err)
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
		return nil, fmt.Errorf("get by week error")
	}

	group, ok := sch.Groups[groupID]
	if !ok {
		slog.Warn("group not found", "group_id", groupID, "err", err)
		return nil, fmt.Errorf("group not found")
	}

	return group, nil
}
