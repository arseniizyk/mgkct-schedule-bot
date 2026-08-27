package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/infrastructure/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TeacherScheduleRepository interface {
	Save(ctx context.Context, name string, week time.Time, schedule *pb.Teacher) error
	GetByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error)
	GetLatest(ctx context.Context, name string) (*pb.Teacher, error)
	GetAllLatest(ctx context.Context) (map[string]*pb.Teacher, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
}

type TeacherParser interface {
	Parse(ctx context.Context) ([]parser.TeacherSchedule, *time.Time, error)
}

type TeacherScheduleService struct {
	log          *slog.Logger
	scheduleRepo TeacherScheduleRepository
	parser       TeacherParser

	teacherHashes map[string][32]byte
	teacherWeeks  map[string]time.Time
}

func NewTeacherScheduleService(log *slog.Logger, scheduleRepo TeacherScheduleRepository, parser TeacherParser) *TeacherScheduleService {
	return &TeacherScheduleService{
		log:           log,
		scheduleRepo:  scheduleRepo,
		parser:        parser,
		teacherHashes: make(map[string][32]byte),
		teacherWeeks:  make(map[string]time.Time),
	}
}

func (ss *TeacherScheduleService) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error) {
	const op = "service.teacher_schedule.TeacherScheduleService.GetTeacherScheduleByWeek"
	log := ss.log.With(
		"operation", op,
		"name", name,
		"week", week,
	)

	t, err := ss.scheduleRepo.GetByWeek(ctx, name, week)
	if err != nil {
		if errors.Is(err, repository.ErrWeekNotFound) {
			log.ErrorContext(ctx, "week not found")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by week", "err", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return t, nil
}

func (ss *TeacherScheduleService) GetTeacherLatestSchedule(ctx context.Context, name string) (*pb.Teacher, error) {
	const op = "service.teacher_schedule.TeacherScheduleService.GetTeacherLatestSchedule"

	log := ss.log.With("operation", op, "name", name)

	t, err := ss.scheduleRepo.GetLatest(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrScheduleNotFound) {
			log.ErrorContext(ctx, "schedule not found in repository", "error", err)
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by latest", "error", err)
		return nil, fmt.Errorf("get by week error: %w", err)
	}

	return t, nil
}

func (ss *TeacherScheduleService) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return ss.scheduleRepo.GetAllTeacherNames(ctx)
}

func (ss *TeacherScheduleService) CheckTeacherScheduleUpdates(ctx context.Context, interval time.Duration) <-chan *entities.UpdatedTeacher {
	const op = "service.teacher_schedule.TeacherScheduleService.CheckTeacherScheduleUpdates"

	log := ss.log.With("operation", op, "interval", interval.Seconds())

	resCh := make(chan *entities.UpdatedTeacher, 5)

	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		defer close(resCh)

		repoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		allLatest, err := ss.scheduleRepo.GetAllLatest(repoCtx)
		cancel()

		if err != nil {
			log.ErrorContext(ctx, "failed to get all latest teacher schedules from repo", "error", err)
		}

		for name, t := range allLatest {
			if h, err := hashJSON(t); err == nil {
				ss.teacherHashes[name] = h
				ss.teacherWeeks[name] = t.GetWeek().AsTime()
			}
		}

		// Первый запуск парсинга сразу при старте
		schedules, _, err := ss.parser.Parse(ctx)
		if err != nil {
			log.ErrorContext(ctx, "failed on parsing teacher schedules on startup", "error", err)
		} else {
			ss.processTeacherSchedules(ctx, schedules, resCh, log)
		}

		for {
			select {
			case <-tick.C:
				schedules, _, err := ss.parser.Parse(ctx)

				if err != nil {
					log.ErrorContext(ctx, "failed on parsing teacher schedules", "error", err)
					continue
				}

				ss.processTeacherSchedules(ctx, schedules, resCh, log)

			case <-ctx.Done():
				log.InfoContext(ctx, "ctx.Done Received")
				return
			}
		}
	}()

	return resCh
}

func (ss *TeacherScheduleService) processTeacherSchedules(ctx context.Context, schedules []parser.TeacherSchedule, resCh chan<- *entities.UpdatedTeacher, log *slog.Logger) {
	for _, ts := range schedules {
		pbTeacher := &pb.Teacher{
			Name: ts.Name,
			Week: timestamppb.New(ts.Week),
			Days: ts.Days,
		}

		newHash, err := hashJSON(pbTeacher)
		if err != nil {
			log.ErrorContext(ctx, "failed to hash teacher", "name", ts.Name, "err", err)
			continue
		}

		oldHash := ss.teacherHashes[ts.Name]
		oldWeek := ss.teacherWeeks[ts.Name]
		if oldHash == newHash {
			continue
		}

		saveCtx, saveCancel := context.WithTimeout(ctx, 5*time.Second)
		if err := ss.scheduleRepo.Save(saveCtx, ts.Name, ts.Week, pbTeacher); err != nil {
			saveCancel()
			log.ErrorContext(ctx, "failed to save teacher schedule", "name", ts.Name, "err", err)
			continue
		}
		saveCancel()

		ss.teacherHashes[ts.Name] = newHash
		ss.teacherWeeks[ts.Name] = ts.Week

		isWeekUpdated := !oldWeek.IsZero() && !oldWeek.Equal(ts.Week)

		log.InfoContext(ctx, "teacher updated", "name", ts.Name, "is_week_updated", isWeekUpdated)
		resCh <- &entities.UpdatedTeacher{
			Teacher:       pbTeacher,
			IsWeekUpdated: isWeekUpdated,
		}
	}
}

func (ss *TeacherScheduleService) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	const op = "service.teacher_schedule.TeacherScheduleService.GetAvailableWeeks"
	log := ss.log.With(
		"operation", op,
		"name", name,
		"week", week.String(),
	)

	navigation, err := ss.scheduleRepo.GetWeeks(ctx, name, week)
	if err != nil {
		if !errors.Is(err, repository.ErrNoAvailableWeeks) {
			log.ErrorContext(ctx, "failed get available weeks", "err", err)
			return entities.WeekNavigation{}, err
		}
		return entities.WeekNavigation{}, nil
	}

	return navigation, nil
}
