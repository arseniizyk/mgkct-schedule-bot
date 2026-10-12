package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

type StudentParser interface {
	Parse(ctx context.Context) (map[time.Time]*entities.Schedule, error)
}

type TeacherParser interface {
	Parse(ctx context.Context) ([]*entities.Teacher, error)
}

type ScraperService struct {
	log           *slog.Logger
	cfg           config.ScraperConfig
	studentParser StudentParser
	teacherParser TeacherParser
	scheduleRepo  ScheduleRepository
	teacherRepo   TeacherScheduleRepository
	scheduleSvc   *ScheduleService

	mu             sync.RWMutex
	latestWeek     time.Time
	groupHashes    map[int32][32]byte
	teacherHashes  map[string][32]byte
	newWeekFoundInTurbo bool
}

func NewScraperService(
	log *slog.Logger,
	cfg config.ScraperConfig,
	studentParser StudentParser,
	teacherParser TeacherParser,
	scheduleRepo ScheduleRepository,
	teacherRepo TeacherScheduleRepository,
	scheduleSvc *ScheduleService,
) *ScraperService {
	return &ScraperService{
		log:           log,
		cfg:           cfg,
		studentParser: studentParser,
		teacherParser: teacherParser,
		scheduleRepo:  scheduleRepo,
		teacherRepo:   teacherRepo,
		scheduleSvc:   scheduleSvc,
		groupHashes:   make(map[int32][32]byte),
		teacherHashes: make(map[string][32]byte),
	}
}

// Start запускает фоновый процесс умного скрапинга и отправляет события в eventCh.
func (s *ScraperService) Start(ctx context.Context, eventCh chan<- Event) {
	log := s.log.With("operation", "service.ScraperService.Start")
	log.Info("Scraper service started")

	// 1. Инициализация из БД при старте
	s.initFromDB(ctx)

	// 2. Первоначальный опрос
	s.scrapeOnce(ctx, eventCh)

	// 3. Основной цикл динамического опроса
	for {
		interval := s.calculateInterval(time.Now())
		log.DebugContext(ctx, "waiting next scrape", "interval", interval.String())

		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "scraper stopped")
			return
		case <-time.After(interval):
			s.scrapeOnce(ctx, eventCh)
		}
	}
}

// calculateInterval вычисляет текущий интервал опроса:
// Пятница 12:00 — Суббота 14:00 (Минское время UTC+3): если новая неделя еще не найдена, опрос каждые 20-30 сек.
// Ночь (22:00 — 08:00): опрос каждые 20 минут.
// Обычное время: раз в минуту.
func (s *ScraperService) calculateInterval(now time.Time) time.Duration {
	minskLoc := time.FixedZone("MSK", 3*3600)
	localNow := now.In(minskLoc)

	weekday := localNow.Weekday()
	hour := localNow.Hour()

	// Пятница 12:00 — Суббота 14:00: турбо-режим ожидания новой недели
	isTurboWindow := (weekday == time.Friday && hour >= 12) || (weekday == time.Saturday && hour < 14)
	if isTurboWindow {
		s.mu.RLock()
		found := s.newWeekFoundInTurbo
		s.mu.RUnlock()

		if !found {
			return s.cfg.PeakInterval
		}
	} else {
		// Вне турбо-окна сбрасываем флаг, чтобы в следующую пятницу турбо снова включился
		s.mu.Lock()
		s.newWeekFoundInTurbo = false
		s.mu.Unlock()
	}

	// Ночное время (22:00 - 08:00) или Воскресенье
	if weekday == time.Sunday || hour >= 22 || hour < 8 {
		return s.cfg.NightInterval
	}

	return s.cfg.Interval
}

func (s *ScraperService) initFromDB(ctx context.Context) {
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	latest, err := s.scheduleRepo.GetLatest(initCtx)
	if err == nil && latest != nil {
		s.mu.Lock()
		for _, g := range latest.Groups {
			if g.Week.After(s.latestWeek) {
				s.latestWeek = g.Week
			}
			if h, err := hashItem(g); err == nil {
				s.groupHashes[g.ID] = h
			}
		}
		s.mu.Unlock()
		s.scheduleSvc.SetCache(latest)
	}

	teachers, err := s.teacherRepo.GetAllLatest(initCtx)
	if err == nil && teachers != nil {
		s.mu.Lock()
		for name, t := range teachers {
			if h, err := hashItem(t); err == nil {
				s.teacherHashes[name] = h
			}
		}
		s.mu.Unlock()
	}
}

func (s *ScraperService) scrapeOnce(ctx context.Context, eventCh chan<- Event) {
	log := s.log.With("operation", "service.ScraperService.scrapeOnce")

	scrapeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// 1. Скрапинг групп
	schedulesByWeek, err := s.studentParser.Parse(scrapeCtx)
	if err != nil {
		log.ErrorContext(ctx, "failed parsing student schedules", "error", err)
	} else {
		s.processStudentSchedules(ctx, schedulesByWeek, eventCh)
	}

	// 2. Скрапинг преподавателей
	teachers, err := s.teacherParser.Parse(scrapeCtx)
	if err != nil {
		log.ErrorContext(ctx, "failed parsing teacher schedules", "error", err)
	} else {
		s.processTeacherSchedules(ctx, teachers, eventCh)
	}
}

func (s *ScraperService) processStudentSchedules(ctx context.Context, schedules map[time.Time]*entities.Schedule, eventCh chan<- Event) {
	log := s.log.With("operation", "service.ScraperService.processStudentSchedules")

	var maxWeek time.Time

	for week, sch := range schedules {
		if week.After(maxWeek) {
			maxWeek = week
		}

		// Сохраняем каждую распарсенную неделю в БД
		if err := s.scheduleRepo.Save(ctx, week, sch); err != nil {
			log.ErrorContext(ctx, "failed saving schedule for week", "week", week, "error", err)
			continue
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверяем появление новой недели: ровно ОДНО событие!
	if !s.latestWeek.IsZero() && maxWeek.After(s.latestWeek) {
		log.InfoContext(ctx, "NEW WEEK DETECTED!", "old_week", s.latestWeek, "new_week", maxWeek)
		s.latestWeek = maxWeek
		s.newWeekFoundInTurbo = true

		eventCh <- Event{
			Type: EventNewWeek,
			Week: maxWeek,
		}
	} else if s.latestWeek.IsZero() {
		s.latestWeek = maxWeek
	}

	// Обновляем кэш последней недели
	if latestSch, ok := schedules[s.latestWeek]; ok {
		s.scheduleSvc.SetCache(latestSch)

		// Проверяем изменения по каждой группе
		for groupID, group := range latestSch.Groups {
			newHash, err := hashItem(group)
			if err != nil {
				continue
			}

			oldHash, exists := s.groupHashes[groupID]
			if !exists || oldHash != newHash {
				s.groupHashes[groupID] = newHash

				// Отправляем событие обновления конкретной группы
				log.InfoContext(ctx, "group schedule updated", "group_id", groupID)
				eventCh <- Event{
					Type:  EventGroupScheduleUpdated,
					Group: group,
				}
			}
		}
	}
}

func (s *ScraperService) processTeacherSchedules(ctx context.Context, teachers []*entities.Teacher, eventCh chan<- Event) {
	log := s.log.With("operation", "service.ScraperService.processTeacherSchedules")

	for _, t := range teachers {
		if err := s.teacherRepo.Save(ctx, t.Name, t.Week, t); err != nil {
			log.ErrorContext(ctx, "failed saving teacher schedule", "name", t.Name, "error", err)
			continue
		}

		newHash, err := hashItem(t)
		if err != nil {
			continue
		}

		s.mu.Lock()
		oldHash, exists := s.teacherHashes[t.Name]
		if !exists || oldHash != newHash {
			s.teacherHashes[t.Name] = newHash
			s.mu.Unlock()

			log.InfoContext(ctx, "teacher schedule updated", "name", t.Name)
			eventCh <- Event{
				Type:    EventTeacherScheduleUpdated,
				Teacher: t,
			}
		} else {
			s.mu.Unlock()
		}
	}
}

func hashItem[T any](item T) ([32]byte, error) {
	var zero [32]byte
	data, err := json.Marshal(item)
	if err != nil {
		return zero, fmt.Errorf("marshal item: %w", err)
	}
	return sha256.Sum256(data), nil
}
