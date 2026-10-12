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

type FacultyParser interface {
	Parse(ctx context.Context) ([]*entities.FacultyMember, error)
}

type ScraperService struct {
	log           *slog.Logger
	cfg           config.ScraperConfig
	studentParser StudentParser
	teacherParser TeacherParser
	scheduleRepo  ScheduleRepository
	teacherRepo   TeacherScheduleRepository
	scheduleSvc   *ScheduleService
	facultyParser FacultyParser
	facultyRepo   FacultyRepository

	mu                  sync.RWMutex
	latestWeek          time.Time
	lastFacultyScrape   time.Time
	groupHashes         map[string][32]byte // key: "<group_id>:<YYYY-MM-DD>"
	teacherHashes       map[string][32]byte // key: "<teacher_name>:<YYYY-MM-DD>"
	newWeekFoundInTurbo bool
	consecutiveErrors   int
	adminAlerted        bool
	lastAlertTime       time.Time
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
		groupHashes:   make(map[string][32]byte),
		teacherHashes: make(map[string][32]byte),
	}
}

func (s *ScraperService) SetFaculty(facultyParser FacultyParser, facultyRepo FacultyRepository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facultyParser = facultyParser
	s.facultyRepo = facultyRepo
}

func (s *ScraperService) scrapeFaculty(ctx context.Context) {
	s.mu.RLock()
	parser := s.facultyParser
	repo := s.facultyRepo
	s.mu.RUnlock()

	if parser == nil || repo == nil {
		return
	}

	members, err := parser.Parse(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed parsing faculty members", "error", err)
		return
	}

	if len(members) > 0 {
		if err := repo.SaveBatch(ctx, members); err != nil {
			s.log.ErrorContext(ctx, "failed saving faculty members batch", "error", err)
			return
		}
		s.log.InfoContext(ctx, "faculty members updated successfully", "count", len(members))
		s.mu.Lock()
		s.lastFacultyScrape = time.Now()
		s.mu.Unlock()
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

	schedules, err := s.scheduleRepo.GetAll(initCtx)
	if err == nil && len(schedules) > 0 {
		s.mu.Lock()
		for week, sch := range schedules {
			if week.After(s.latestWeek) {
				s.latestWeek = week
			}
			for _, g := range sch.Groups {
				key := fmt.Sprintf("%d:%s", g.ID, g.Week.Format("2006-01-02"))
				if h, err := hashItem(g); err == nil {
					s.groupHashes[key] = h
				}
			}
		}
		s.mu.Unlock()

		if latestSch, ok := schedules[s.latestWeek]; ok {
			s.scheduleSvc.SetCache(latestSch)
		}
	}

	teachers, err := s.teacherRepo.GetAll(initCtx)
	if err == nil && len(teachers) > 0 {
		s.mu.Lock()
		for _, t := range teachers {
			key := fmt.Sprintf("%s:%s", t.Name, t.Week.Format("2006-01-02"))
			if h, err := hashItem(t); err == nil {
				s.teacherHashes[key] = h
			}
		}
		s.mu.Unlock()
	}
}

func (s *ScraperService) scrapeOnce(ctx context.Context, eventCh chan<- Event) {
	log := s.log.With("operation", "service.ScraperService.scrapeOnce")

	scrapeCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	// 1. Скрапинг групп
	schedulesByWeek, studentErr := s.studentParser.Parse(scrapeCtx)
	if studentErr != nil {
		log.ErrorContext(ctx, "failed parsing student schedules", "error", studentErr)
	} else {
		s.processStudentSchedules(ctx, schedulesByWeek, eventCh)
	}

	// 2. Скрапинг преподавателей
	teachers, teacherErr := s.teacherParser.Parse(scrapeCtx)
	if teacherErr != nil {
		log.ErrorContext(ctx, "failed parsing teacher schedules", "error", teacherErr)
	} else {
		s.processTeacherSchedules(ctx, teachers, eventCh)
	}

	// Отслеживание сбоев скрапинга и алертов
	var scrapeErr error
	if studentErr != nil && teacherErr != nil {
		scrapeErr = fmt.Errorf("students: %w; teachers: %w", studentErr, teacherErr)
	}

	s.mu.Lock()
	if scrapeErr != nil {
		s.consecutiveErrors++
		if s.consecutiveErrors >= 5 && (!s.adminAlerted || time.Since(s.lastAlertTime) > 30*time.Minute) {
			s.adminAlerted = true
			s.lastAlertTime = time.Now()
			fails := s.consecutiveErrors
			s.mu.Unlock()
			eventCh <- Event{
				Type:             EventScraperError,
				Err:              scrapeErr,
				ConsecutiveFails: fails,
			}
		} else {
			s.mu.Unlock()
		}
	} else {
		wasAlerted := s.adminAlerted
		s.adminAlerted = false
		s.consecutiveErrors = 0
		s.mu.Unlock()

		if wasAlerted {
			eventCh <- Event{
				Type: EventScraperRecovered,
			}
		}
	}

	// 3. Скрапинг педсостава (раз в сутки или при первом старте)
	s.mu.RLock()
	needFaculty := s.facultyParser != nil && (s.lastFacultyScrape.IsZero() || time.Since(s.lastFacultyScrape) > 24*time.Hour)
	s.mu.RUnlock()

	if needFaculty {
		go s.scrapeFaculty(ctx)
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
	}

	// Проверяем изменения по каждой группе во всех распарсенных неделях
	for _, sch := range schedules {
		for groupID, group := range sch.Groups {
			newHash, err := hashItem(group)
			if err != nil {
				continue
			}

			key := fmt.Sprintf("%d:%s", groupID, group.Week.Format("2006-01-02"))
			oldHash, exists := s.groupHashes[key]
			s.groupHashes[key] = newHash

			// Уведомление отправляется ТОЛЬКО если расписание уже существовало и его содержимое реально изменилось!
			if exists && oldHash != newHash {
				log.InfoContext(ctx, "group schedule updated", "group_id", groupID, "week", group.Week)
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

		key := fmt.Sprintf("%s:%s", t.Name, t.Week.Format("2006-01-02"))

		s.mu.Lock()
		oldHash, exists := s.teacherHashes[key]
		s.teacherHashes[key] = newHash
		s.mu.Unlock()

		// Уведомление отправляется ТОЛЬКО если расписание уже существовало и его содержимое реально изменилось!
		if exists && oldHash != newHash {
			log.InfoContext(ctx, "teacher schedule updated", "name", t.Name, "week", t.Week)
			eventCh <- Event{
				Type:    EventTeacherScheduleUpdated,
				Teacher: t,
			}
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
