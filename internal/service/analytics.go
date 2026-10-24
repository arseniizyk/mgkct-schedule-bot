package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type AnalyticsService struct {
	log         *slog.Logger
	schedRepo   ScheduleRepository
	teacherRepo TeacherScheduleRepository
	dict        SubjectDictionary
	nowFunc     func() time.Time
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

func (s *AnalyticsService) SetNowFunc(f func() time.Time) {
	s.nowFunc = f
}

func (s *AnalyticsService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
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
		targetWeek := EffectiveEndingsWeek(s.now())
		sch, err = s.schedRepo.GetByWeek(ctx, targetWeek)
		if err != nil || sch == nil {
			sch, err = s.schedRepo.GetLatest(ctx)
		}
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
func (s *AnalyticsService) GetEndings(ctx context.Context, dayIdx int, week time.Time) (*entities.EndingsStat, error) {
	var sch *entities.Schedule
	var err error
	if week.IsZero() {
		targetWeek := EffectiveEndingsWeek(s.now())
		sch, err = s.schedRepo.GetByWeek(ctx, targetWeek)
		if err != nil || sch == nil {
			sch, err = s.schedRepo.GetLatest(ctx)
		}
	} else {
		sch, err = s.schedRepo.GetByWeek(ctx, week)
	}
	if err != nil {
		return nil, fmt.Errorf("analytics: get schedule for endings: %w", err)
	}

	stat := &entities.EndingsStat{
		DayIndex:               dayIdx,
		Pairs:                  make(map[int][]entities.GroupEnding),
		DayOff:                 make([]entities.GroupEnding, 0),
		Total:                  len(sch.Groups),
		StudentsByPair:         make(map[int]int),
		KazintsaStudentsByPair: make(map[int]int),
		KnorinaStudentsByPair:  make(map[int]int),
	}

	for _, grp := range sch.Groups {
		stat.TotalStudents += entities.CalculateSubgroupStudents(grp.ID, 0)

		if dayIdx < 0 || dayIdx >= len(grp.Days) {
			continue
		}
		if stat.DayName == "" {
			stat.DayName = grp.Days[dayIdx].Name
		}

		endings := calculateGroupDayEndings(grp.Days[dayIdx].Subjects)
		for sg, pairNum := range endings {
			campus := resolveEndingCampus(grp, dayIdx, sg)
			item := entities.GroupEnding{
				GroupID:  grp.ID,
				Subgroup: sg,
				Campus:   campus,
			}
			students := entities.CalculateSubgroupStudents(grp.ID, sg)
			if pairNum == 0 {
				stat.DayOff = append(stat.DayOff, item)
				stat.DayOffStudents += students
			} else {
				stat.Pairs[pairNum] = append(stat.Pairs[pairNum], item)
				stat.StudentsByPair[pairNum] += students
				if campus == entities.CampusKnorina {
					stat.KnorinaTotalStudents += students
					stat.KnorinaStudentsByPair[pairNum] += students
				} else {
					stat.KazintsaTotalStudents += students
					stat.KazintsaStudentsByPair[pairNum] += students
				}
			}
		}
	}

	sortGroupEndings := func(slice []entities.GroupEnding) {
		sort.Slice(slice, func(i, j int) bool {
			if slice[i].GroupID != slice[j].GroupID {
				return slice[i].GroupID < slice[j].GroupID
			}
			return slice[i].Subgroup < slice[j].Subgroup
		})
	}

	// Сортировка списков групп для детерминированного вывода
	sortGroupEndings(stat.DayOff)
	for k := range stat.Pairs {
		sortGroupEndings(stat.Pairs[k])
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

// parsePairSubgroup извлекает номер подгруппы (1 или 2) из названия пары или поля group.
// Возвращает 0, если пара общая для всей группы (или спецгруппа вроде 3. ФЗКиЗ).
func parsePairSubgroup(name, groupField string) int {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && unicode.IsDigit(rune(name[0])) && name[1] == '.' {
		sg := int(name[0] - '0')
		if sg == 1 || sg == 2 {
			return sg
		}
		return 0
	}

	lower := strings.ToLower(groupField)
	if strings.Contains(lower, "1 п/г") || strings.Contains(lower, "1 п/гр") || strings.TrimSpace(groupField) == "1" {
		return 1
	}
	if strings.Contains(lower, "2 п/г") || strings.Contains(lower, "2 п/гр") || strings.TrimSpace(groupField) == "2" {
		return 2
	}

	return 0
}

// IsKnorinaCabinet проверяет, относится ли кабинет к корпусу на ул. Кнорина (префикс "Кн-", "Кн.", "Кн ").
func IsKnorinaCabinet(cabinet string) bool {
	c := strings.ToLower(strings.TrimSpace(cabinet))
	return strings.HasPrefix(c, "кн")
}

// resolveEndingCampus определяет корпус (Казинца или Кнорина) для группы/подгруппы на указанный день
func resolveEndingCampus(grp *entities.Group, dayIdx int, sg int) string {
	if grp == nil || dayIdx < 0 || dayIdx >= len(grp.Days) {
		return entities.CampusKazintsa
	}
	day := grp.Days[dayIdx]

	var sgCabinets []string
	var dayCabinets []string

	for _, subj := range day.Subjects {
		if subj == nil || subj.IsEmpty {
			continue
		}
		for _, p := range subj.Pairs {
			if p == nil {
				continue
			}
			cab := strings.TrimSpace(p.Class)
			if cab == "" || cab == "-" {
				continue
			}
			dayCabinets = append(dayCabinets, cab)
			name := strings.TrimSpace(p.Name)
			pairSg := parsePairSubgroup(name, p.Group)
			if sg == 0 || pairSg == 0 || pairSg == sg {
				sgCabinets = append(sgCabinets, cab)
			}
		}
	}

	if slices.ContainsFunc(sgCabinets, IsKnorinaCabinet) {
		return entities.CampusKnorina
	}
	if len(sgCabinets) > 0 {
		return entities.CampusKazintsa
	}

	if slices.ContainsFunc(dayCabinets, IsKnorinaCabinet) {
		return entities.CampusKnorina
	}
	if len(dayCabinets) > 0 {
		return entities.CampusKazintsa
	}

	return resolveGroupWeekCampus(grp)
}

func resolveGroupWeekCampus(grp *entities.Group) string {
	knCnt := 0
	kazCnt := 0
	for _, d := range grp.Days {
		if d == nil {
			continue
		}
		for _, subj := range d.Subjects {
			if subj == nil || subj.IsEmpty {
				continue
			}
			for _, p := range subj.Pairs {
				if p == nil {
					continue
				}
				cab := strings.TrimSpace(p.Class)
				if cab == "" || cab == "-" {
					continue
				}
				if IsKnorinaCabinet(cab) {
					knCnt++
				} else {
					kazCnt++
				}
			}
		}
	}

	if knCnt > kazCnt {
		return entities.CampusKnorina
	}
	return entities.CampusKazintsa
}

// calculateGroupDayEndings определяет номера последних пар (1-based, 0 для выходного) для подгрупп группы.
// Если группа не делится на подгруппы (или все подгруппы заканчивают одинаково), возвращается map[int]int{0: pairNum}.
// Если подгруппы заканчивают в разное время (например, 1-я в 5, 2-я в 3), возвращается map[int]int{1: 5, 2: 3}.
func calculateGroupDayEndings(subjects []*entities.Subject) map[int]int {
	maxSubgroup := 0

	for _, subj := range subjects {
		if subj == nil || subj.IsEmpty {
			continue
		}
		for _, p := range subj.Pairs {
			if p == nil {
				continue
			}
			name := strings.TrimSpace(p.Name)
			if name == "" && p.Group == "" {
				continue
			}
			sg := parsePairSubgroup(name, p.Group)
			if sg > maxSubgroup {
				maxSubgroup = sg
			}
		}
	}

	if maxSubgroup == 0 {
		lastPair := findLastSubjectInDay(subjects)
		if lastPair == -1 {
			return map[int]int{0: 0}
		}
		return map[int]int{0: lastPair + 1}
	}

	numSubgroups := max(2, maxSubgroup)

	endings := make(map[int]int, numSubgroups)
	for sg := 1; sg <= numSubgroups; sg++ {
		lastPair := -1
		for i, subj := range slices.Backward(subjects) {
			if subj == nil || subj.IsEmpty || len(subj.Pairs) == 0 {
				continue
			}

			hasPairForSg := false
			for _, p := range subj.Pairs {
				if p == nil {
					continue
				}
				name := strings.TrimSpace(p.Name)
				if name == "" && p.Group == "" {
					continue
				}

				pairSg := parsePairSubgroup(name, p.Group)
				if pairSg == sg || pairSg == 0 {
					hasPairForSg = true
					break
				}
			}

			if hasPairForSg {
				lastPair = i
				break
			}
		}

		if lastPair == -1 {
			endings[sg] = 0
		} else {
			endings[sg] = lastPair + 1
		}
	}

	firstEnding := endings[1]
	allSame := true
	for sg := 2; sg <= numSubgroups; sg++ {
		if endings[sg] != firstEnding {
			allSame = false
			break
		}
	}

	if allSame {
		return map[int]int{0: firstEnding}
	}

	return endings
}

// EffectiveEndingsWeek возвращает дату понедельника недели для показа окончаний пар
func EffectiveEndingsWeek(now time.Time) time.Time {
	return EffectiveScheduleWeek(now)
}

// EffectiveEndingsDayIndexAt вычисляет индекс дня для показа окончаний пар для указанного времени
func EffectiveEndingsDayIndexAt(now time.Time) int {
	localNow := now.In(minskLoc)
	today := (int(localNow.Weekday()) + 6) % 7 // Пн=0..Вс=6
	if today > 5 {
		return 0 // Воскресенье -> Понедельник
	}
	// Если уже вечер (после 19:00), показываем следующий день
	if localNow.Hour() >= 19 {
		if today == 5 {
			return 0
		}
		return today + 1
	}
	return today
}

// EffectiveEndingsDayIndex вычисляет индекс дня для показа окончаний пар
func (s *AnalyticsService) EffectiveEndingsDayIndex() int {
	return EffectiveEndingsDayIndexAt(s.now())
}

// EffectiveEndingsDayIndex вычисляет индекс дня для показа окончаний пар
func EffectiveEndingsDayIndex() int {
	return EffectiveEndingsDayIndexAt(time.Now())
}
