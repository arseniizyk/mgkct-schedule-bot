package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type AnalyticsService struct {
	log         *slog.Logger
	schedRepo   ScheduleRepository
	teacherRepo TeacherScheduleRepository
	dict        SubjectDictionary
}

func NewAnalyticsService(
	log *slog.Logger,
	schedRepo ScheduleRepository,
	teacherRepo TeacherScheduleRepository,
	dict SubjectDictionary,
) *AnalyticsService {
	return &AnalyticsService{
		log:         log,
		schedRepo:   schedRepo,
		teacherRepo: teacherRepo,
		dict:        dict,
	}
}

// GetAllCabinets возвращает список всех уникальных кабинетов, упорядоченных по имени
func (s *AnalyticsService) GetAllCabinets(ctx context.Context) ([]string, error) {
	sch, err := s.schedRepo.GetLatest(ctx)
	if err != nil {
		return nil, fmt.Errorf("analytics: get latest schedule: %w", err)
	}

	cabMap := make(map[string]struct{})
	for _, grp := range sch.Groups {
		for _, day := range grp.Days {
			for _, subj := range day.Subjects {
				for _, p := range subj.Pairs {
					cls := strings.TrimSpace(p.Class)
					if cls != "" && cls != "-" {
						cabMap[cls] = struct{}{}
					}
				}
			}
		}
	}

	cabinets := make([]string, 0, len(cabMap))
	for c := range cabMap {
		cabinets = append(cabinets, c)
	}
	sort.Strings(cabinets)

	return cabinets, nil
}

// GetCabinetSchedule возвращает расписание занятости конкретного кабинета на неделю
func (s *AnalyticsService) GetCabinetSchedule(ctx context.Context, query string, week time.Time) (*entities.CabinetSchedule, []string, error) {
	allCabinets, err := s.GetAllCabinets(ctx)
	if err != nil {
		return nil, nil, err
	}

	matchedCabinet, candidates := matchCabinet(query, allCabinets)
	if matchedCabinet == "" {
		return nil, candidates, nil
	}

	var sch *entities.Schedule
	if week.IsZero() {
		sch, err = s.schedRepo.GetLatest(ctx)
	} else {
		sch, err = s.schedRepo.GetByWeek(ctx, week)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("analytics: get schedule for cabinet: %w", err)
	}

	daysMap := make(map[int]*entities.CabinetDaySchedule)
	var maxDays int

	for _, grp := range sch.Groups {
		if len(grp.Days) > maxDays {
			maxDays = len(grp.Days)
		}
		for dayIdx, day := range grp.Days {
			daySched, exists := daysMap[dayIdx]
			if !exists {
				daySched = &entities.CabinetDaySchedule{
					DayIndex: dayIdx,
					DayName:  day.Name,
					Bookings: make([]*entities.CabinetBooking, 0),
				}
				daysMap[dayIdx] = daySched
			}

			for pairIdx, subj := range day.Subjects {
				for _, p := range subj.Pairs {
					if strings.EqualFold(strings.TrimSpace(p.Class), matchedCabinet) {
						var fullSubj string
						if s.dict != nil {
							_, fullSubj, _ = s.dict.Resolve(p.Name)
						}
						daySched.Bookings = append(daySched.Bookings, &entities.CabinetBooking{
							DayIndex:    dayIdx,
							DayName:     day.Name,
							PairIndex:   pairIdx + 1,
							GroupID:     grp.ID,
							Subject:     p.Name,
							SubjectFull: fullSubj,
							Type:        p.Type,
							Teacher:     p.Group,
							Class:       p.Class,
						})
					}
				}
			}
		}
	}

	days := make([]*entities.CabinetDaySchedule, 0, maxDays)
	for i := range maxDays {
		if d, ok := daysMap[i]; ok {
			// Сортировка пар по возрастанию номера
			sort.Slice(d.Bookings, func(a, b int) bool {
				if d.Bookings[a].PairIndex != d.Bookings[b].PairIndex {
					return d.Bookings[a].PairIndex < d.Bookings[b].PairIndex
				}
				return d.Bookings[a].GroupID < d.Bookings[b].GroupID
			})
			days = append(days, d)
		}
	}

	var schWeek time.Time
	for _, g := range sch.Groups {
		schWeek = g.Week
		break
	}

	return &entities.CabinetSchedule{
		Cabinet: matchedCabinet,
		Week:    schWeek,
		Days:    days,
	}, nil, nil
}

// GetEndings собирает аналитику окончания занятий групп на указанный день недели
func (s *AnalyticsService) GetEndings(ctx context.Context, dayIdx int) (*entities.EndingsStat, error) {
	sch, err := s.schedRepo.GetLatest(ctx)
	if err != nil {
		return nil, fmt.Errorf("analytics: get latest schedule: %w", err)
	}

	stat := &entities.EndingsStat{
		DayIndex: dayIdx,
		Pairs:    make(map[int][]int32),
		DayOff:   make([]int32, 0),
		Total:    len(sch.Groups),
	}

	for _, grp := range sch.Groups {
		if dayIdx < 0 || dayIdx >= len(grp.Days) {
			continue
		}
		if stat.DayName == "" {
			stat.DayName = grp.Days[dayIdx].Name
		}

		lastPair := findLastSubjectInDay(grp.Days[dayIdx].Subjects)
		if lastPair == -1 {
			stat.DayOff = append(stat.DayOff, grp.ID)
		} else {
			pairNum := lastPair + 1
			stat.Pairs[pairNum] = append(stat.Pairs[pairNum], grp.ID)
		}
	}

	// Сортировка списков групп для детерминированного вывода
	slices.Sort(stat.DayOff)
	for k := range stat.Pairs {
		slices.Sort(stat.Pairs[k])
	}

	return stat, nil
}

// GetGroupStats собирает статистику проведенных/запланированных пар для группы
func (s *AnalyticsService) GetGroupStats(ctx context.Context, groupID int32, week time.Time) ([]entities.SubjectStat, int, error) {
	var sch *entities.Schedule
	var err error
	if week.IsZero() {
		sch, err = s.schedRepo.GetLatest(ctx)
	} else {
		sch, err = s.schedRepo.GetByWeek(ctx, week)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("analytics: get group schedule: %w", err)
	}

	grp, ok := sch.Groups[groupID]
	if !ok {
		return nil, 0, domainerr.ErrGroupNotFound
	}

	type subjAgg struct {
		name  string
		count int
		types map[string]int
	}
	agg := make(map[string]*subjAgg)
	total := 0

	for _, day := range grp.Days {
		for _, subj := range day.Subjects {
			if subj.IsEmpty {
				continue
			}
			for _, p := range subj.Pairs {
				if p.Name == "" {
					continue
				}
				cleanName := p.Name
				if s.dict != nil {
					cleanName, _, _ = s.dict.Resolve(p.Name)
				}
				item, exists := agg[cleanName]
				if !exists {
					item = &subjAgg{name: cleanName, types: make(map[string]int)}
					agg[cleanName] = item
				}
				item.count++
				total++
				if p.Type != "" {
					item.types[p.Type]++
				}
			}
		}
	}

	stats := make([]entities.SubjectStat, 0, len(agg))
	for name, item := range agg {
		var full string
		if s.dict != nil {
			full, _ = s.dict.Lookup(name)
		}
		stats = append(stats, entities.SubjectStat{
			Subject:     name,
			SubjectFull: full,
			Count:       item.count,
			Types:       item.types,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Subject < stats[j].Subject
	})

	return stats, total, nil
}

// GetTeacherStats собирает статистику нагрузки для преподавателя
func (s *AnalyticsService) GetTeacherStats(ctx context.Context, teacherName string, week time.Time) ([]entities.SubjectStat, int, error) {
	var t *entities.Teacher
	var err error
	if week.IsZero() {
		t, err = s.teacherRepo.GetLatest(ctx, teacherName)
	} else {
		t, err = s.teacherRepo.GetByWeek(ctx, teacherName, week)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("analytics: get teacher schedule: %w", err)
	}

	type subjAgg struct {
		name  string
		count int
		types map[string]int
	}
	agg := make(map[string]*subjAgg)
	total := 0

	for _, day := range t.Days {
		for _, subj := range day.Subjects {
			if subj.IsEmpty {
				continue
			}
			for _, p := range subj.Pairs {
				if p.Name == "" {
					continue
				}
				cleanName := p.Name
				if s.dict != nil {
					cleanName, _, _ = s.dict.Resolve(p.Name)
				}
				item, exists := agg[cleanName]
				if !exists {
					item = &subjAgg{name: cleanName, types: make(map[string]int)}
					agg[cleanName] = item
				}
				item.count++
				total++
				if p.Type != "" {
					item.types[p.Type]++
				}
			}
		}
	}

	stats := make([]entities.SubjectStat, 0, len(agg))
	for name, item := range agg {
		var full string
		if s.dict != nil {
			full, _ = s.dict.Lookup(name)
		}
		stats = append(stats, entities.SubjectStat{
			Subject:     name,
			SubjectFull: full,
			Count:       item.count,
			Types:       item.types,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Subject < stats[j].Subject
	})

	return stats, total, nil
}

func matchCabinet(query string, all []string) (exact string, candidates []string) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return "", nil
	}

	// 1. Точное совпадение без учета регистра
	for _, c := range all {
		if strings.EqualFold(strings.TrimSpace(c), query) {
			return c, nil
		}
	}

	// 2. Нормализованное совпадение (без дефисов, точек и пробелов)
	cleanQ := normalizeClassQuery(q)
	for _, c := range all {
		cleanC := normalizeClassQuery(c)
		if cleanC == cleanQ {
			return c, nil
		}
	}

	// 3. Частичное совпадение
	for _, c := range all {
		cleanC := normalizeClassQuery(c)
		if strings.Contains(cleanC, cleanQ) || strings.Contains(strings.ToLower(c), q) {
			candidates = append(candidates, c)
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	return "", candidates
}

func normalizeClassQuery(s string) string {
	r := strings.NewReplacer("-", "", " ", "", ".", "", "(лаб)", "", "(к)", "")
	return strings.ToLower(r.Replace(s))
}

func findLastSubjectInDay(subjects []*entities.Subject) int {
	for i, s := range slices.Backward(subjects) {
		if s != nil && !s.IsEmpty && len(s.Pairs) > 0 {
			hasRealPair := false
			for _, p := range s.Pairs {
				if p.Name != "" || p.Group != "" {
					hasRealPair = true
					break
				}
			}
			if hasRealPair {
				return i
			}
		}
	}
	return -1
}

// EffectiveEndingsDayIndex вычисляет индекс дня для показа окончаний пар
func (s *AnalyticsService) EffectiveEndingsDayIndex() int {
	return EffectiveEndingsDayIndex()
}

// EffectiveEndingsDayIndex вычисляет индекс дня для показа окончаний пар
func EffectiveEndingsDayIndex() int {
	now := time.Now()
	today := (int(now.Weekday()) + 6) % 7 // Пн=0..Вс=6
	if today > 5 {
		return 0 // Воскресенье -> Понедельник
	}
	// Если уже вечер (после 19:00), показываем следующий день
	if now.Hour() >= 19 {
		if today == 5 {
			return 0
		}
		return today + 1
	}
	return today
}
