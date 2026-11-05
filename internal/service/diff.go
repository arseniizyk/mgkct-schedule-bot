package service

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

var (
	minskLoc = timeutil.MinskLocation

	dayNames = [6]string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

	pairStartTimes = map[int][2]int{
		1: {9, 0},
		2: {10, 50},
		3: {13, 0},
		4: {14, 50},
		5: {16, 40},
		6: {18, 30},
	}
)

type pairComparatorFunc func(oldPairs, newPairs []*entities.Pair) []entities.SubgroupDiff

// CurrentWeekMonday возвращает полночь понедельника (UTC+3 / Europe/Minsk) для недели, содержащей указанное время.
func CurrentWeekMonday(now time.Time) time.Time {
	return timeutil.CurrentWeekMonday(now)
}

// EffectiveScheduleWeek возвращает дату понедельника недели для отображения актуального расписания.
// До 19:00 субботы возвращается текущая неделя.
// Начиная с 19:00 субботы и в воскресенье возвращается следующая неделя.
func EffectiveScheduleWeek(now time.Time) time.Time {
	localNow := now.In(minskLoc)
	today := (int(localNow.Weekday()) + 6) % 7 // Пн=0..Вс=6
	currentMonday := CurrentWeekMonday(localNow)

	if today > 5 || (today == 5 && localNow.Hour() >= 19) {
		return currentMonday.AddDate(0, 0, 7)
	}

	return currentMonday
}

// ComputeGroupDiff вычисляет точечные изменения в расписании учебной группы с учетом фильтрации прошедшего времени.
func ComputeGroupDiff(oldGroup, newGroup *entities.Group, now time.Time) *entities.ScheduleDiff {
	if newGroup == nil && oldGroup == nil {
		return nil
	}

	var targetWeek time.Time
	var groupID int32
	if newGroup != nil {
		targetWeek = newGroup.Week
		groupID = newGroup.ID
	} else {
		targetWeek = oldGroup.Week
		groupID = oldGroup.ID
	}

	return computeScheduleDiff(
		fmt.Sprintf("Группа %d", groupID),
		targetWeek,
		groupDays(oldGroup),
		groupDays(newGroup),
		now,
		compareGroupPairs,
	)
}

// ComputeTeacherDiff вычисляет точечные изменения в расписании преподавателя с учетом фильтрации прошедшего времени.
func ComputeTeacherDiff(oldTeacher, newTeacher *entities.Teacher, now time.Time) *entities.ScheduleDiff {
	if newTeacher == nil && oldTeacher == nil {
		return nil
	}

	var targetWeek time.Time
	var teacherName string
	if newTeacher != nil {
		targetWeek = newTeacher.Week
		teacherName = newTeacher.Name
	} else {
		targetWeek = oldTeacher.Week
		teacherName = oldTeacher.Name
	}

	return computeScheduleDiff(
		teacherName,
		targetWeek,
		teacherDays(oldTeacher),
		teacherDays(newTeacher),
		now,
		compareTeacherPairs,
	)
}

func computeScheduleDiff(
	title string,
	targetWeek time.Time,
	oldDays, newDays []*entities.Day,
	now time.Time,
	compareFn pairComparatorFunc,
) *entities.ScheduleDiff {
	if now.IsZero() {
		now = timeutil.Now()
	}
	nowMinsk := now.In(minskLoc)
	currentWeekMonday := CurrentWeekMonday(nowMinsk)

	weekMonday := time.Date(targetWeek.Year(), targetWeek.Month(), targetWeek.Day(), 0, 0, 0, 0, minskLoc)
	if weekMonday.Before(currentWeekMonday) {
		return nil
	}

	isCurrentWeek := weekMonday.Equal(currentWeekMonday)
	todayDate := time.Date(nowMinsk.Year(), nowMinsk.Month(), nowMinsk.Day(), 0, 0, 0, 0, minskLoc)

	maxDays := maxDayCount(oldDays, newDays)
	var daysDiff []entities.DayDiff

	for dayIdx := range maxDays {
		dayDate := weekMonday.AddDate(0, 0, dayIdx)
		if isCurrentWeek && dayDate.Before(todayDate) {
			continue
		}

		dayDiff := computeDayDiff(oldDays, newDays, dayIdx, dayDate, todayDate, isCurrentWeek, nowMinsk, compareFn)
		if dayDiff != nil {
			daysDiff = append(daysDiff, *dayDiff)
		}
	}

	if len(daysDiff) == 0 {
		return nil
	}

	return &entities.ScheduleDiff{
		Title: title,
		Week:  weekMonday,
		Days:  daysDiff,
	}
}

func computeDayDiff(
	oldDays, newDays []*entities.Day,
	dayIdx int,
	dayDate, todayDate time.Time,
	isCurrentWeek bool,
	nowMinsk time.Time,
	compareFn pairComparatorFunc,
) *entities.DayDiff {
	dayName := getDayName(oldDays, newDays, dayIdx)

	var oldSubjects, newSubjects []*entities.Subject
	if dayIdx < len(oldDays) && oldDays[dayIdx] != nil {
		oldSubjects = oldDays[dayIdx].Subjects
	}
	if dayIdx < len(newDays) && newDays[dayIdx] != nil {
		newSubjects = newDays[dayIdx].Subjects
	}

	maxPairs := max(len(oldSubjects), len(newSubjects))
	var pairDiffs []entities.PairDiff

	for pIdx := range maxPairs {
		pairNum := pIdx + 1
		if isCurrentWeek && dayDate.Equal(todayDate) && isPairAlreadyStarted(pairNum, nowMinsk) {
			continue
		}

		var oldSubj, newSubj *entities.Subject
		if pIdx < len(oldSubjects) {
			oldSubj = oldSubjects[pIdx]
		}
		if pIdx < len(newSubjects) {
			newSubj = newSubjects[pIdx]
		}

		oldPairs := extractNonEmptyPairs(oldSubj)
		newPairs := extractNonEmptyPairs(newSubj)

		sgDiffs := compareFn(oldPairs, newPairs)
		if len(sgDiffs) > 0 {
			pairDiffs = append(pairDiffs, entities.PairDiff{
				Number:        pairNum,
				SubgroupDiffs: sgDiffs,
			})
		}
	}

	if len(pairDiffs) == 0 {
		return nil
	}

	return &entities.DayDiff{
		DayName:  dayName,
		DayIndex: dayIdx,
		Date:     dayDate,
		Pairs:    pairDiffs,
	}
}

func isPairAlreadyStarted(pairNum int, nowMinsk time.Time) bool {
	startTime, ok := pairStartTimes[pairNum]
	if !ok {
		return false
	}
	pairStartTime := time.Date(nowMinsk.Year(), nowMinsk.Month(), nowMinsk.Day(), startTime[0], startTime[1], 0, 0, minskLoc)
	return !nowMinsk.Before(pairStartTime)
}

func compareGroupPairs(oldPairs, newPairs []*entities.Pair) []entities.SubgroupDiff {
	if len(oldPairs) == 0 && len(newPairs) == 0 {
		return nil
	}
	if len(oldPairs) == 0 {
		return makeAllAddedDiffs(newPairs)
	}
	if len(newPairs) == 0 {
		return makeAllRemovedDiffs(oldPairs)
	}

	oldHasSubgroups := hasSubgroups(oldPairs)
	newHasSubgroups := hasSubgroups(newPairs)

	switch {
	case !oldHasSubgroups && newHasSubgroups, oldHasSubgroups && !newHasSubgroups:
		return handleSubgroupTransition(oldPairs, newPairs)
	case oldHasSubgroups && newHasSubgroups:
		return compareSubgroups(oldPairs, newPairs)
	default:
		return comparePositionalPairs(oldPairs, newPairs)
	}
}

func makeAllAddedDiffs(pairs []*entities.Pair) []entities.SubgroupDiff {
	var diffs []entities.SubgroupDiff
	for _, np := range pairs {
		diffs = append(diffs, entities.SubgroupDiff{
			Subgroup: extractSubgroupFromPair(np),
			Type:     entities.ChangeTypeAdded,
			NewPair:  np,
		})
	}
	return diffs
}

func makeAllRemovedDiffs(pairs []*entities.Pair) []entities.SubgroupDiff {
	var diffs []entities.SubgroupDiff
	for _, op := range pairs {
		diffs = append(diffs, entities.SubgroupDiff{
			Subgroup: extractSubgroupFromPair(op),
			Type:     entities.ChangeTypeRemoved,
			OldPair:  op,
		})
	}
	return diffs
}

func handleSubgroupTransition(oldPairs, newPairs []*entities.Pair) []entities.SubgroupDiff {
	var diffs []entities.SubgroupDiff
	for _, op := range oldPairs {
		diffs = append(diffs, entities.SubgroupDiff{
			Subgroup: extractSubgroupFromPair(op),
			Type:     entities.ChangeTypeRemoved,
			OldPair:  op,
		})
	}
	for _, np := range newPairs {
		diffs = append(diffs, entities.SubgroupDiff{
			Subgroup: extractSubgroupFromPair(np),
			Type:     entities.ChangeTypeAdded,
			NewPair:  np,
		})
	}
	return diffs
}

func compareSubgroups(oldPairs, newPairs []*entities.Pair) []entities.SubgroupDiff {
	oldBySg := make(map[string]*entities.Pair)
	newBySg := make(map[string]*entities.Pair)
	var sgKeys []string

	for _, p := range oldPairs {
		sg := extractSubgroupFromPair(p)
		if _, exists := oldBySg[sg]; exists {
			return comparePositionalPairs(oldPairs, newPairs)
		}
		oldBySg[sg] = p
		if !slices.Contains(sgKeys, sg) {
			sgKeys = append(sgKeys, sg)
		}
	}
	for _, p := range newPairs {
		sg := extractSubgroupFromPair(p)
		if _, exists := newBySg[sg]; exists {
			return comparePositionalPairs(oldPairs, newPairs)
		}
		newBySg[sg] = p
		if !slices.Contains(sgKeys, sg) {
			sgKeys = append(sgKeys, sg)
		}
	}

	slices.Sort(sgKeys)
	var diffs []entities.SubgroupDiff

	for _, sg := range sgKeys {
		op, opExists := oldBySg[sg]
		np, npExists := newBySg[sg]

		switch {
		case opExists && npExists:
			if !isSamePair(op, np) {
				diffs = append(diffs, entities.SubgroupDiff{
					Subgroup: sg,
					Type:     entities.ChangeTypeModified,
					OldPair:  op,
					NewPair:  np,
				})
			}
		case opExists:
			diffs = append(diffs, entities.SubgroupDiff{
				Subgroup: sg,
				Type:     entities.ChangeTypeRemoved,
				OldPair:  op,
			})
		case npExists:
			diffs = append(diffs, entities.SubgroupDiff{
				Subgroup: sg,
				Type:     entities.ChangeTypeAdded,
				NewPair:  np,
			})
		}
	}
	return diffs
}

func compareTeacherPairs(oldPairs, newPairs []*entities.Pair) []entities.SubgroupDiff {
	if len(oldPairs) == 0 && len(newPairs) == 0 {
		return nil
	}
	if len(oldPairs) == 0 {
		return makeTeacherDiffs(newPairs, entities.ChangeTypeAdded)
	}
	if len(newPairs) == 0 {
		return makeTeacherDiffs(oldPairs, entities.ChangeTypeRemoved)
	}
	return comparePositionalPairs(oldPairs, newPairs)
}

func comparePositionalPairs(oldPairs, newPairs []*entities.Pair) []entities.SubgroupDiff {
	if len(oldPairs) == 1 && len(newPairs) == 1 {
		if !isSamePair(oldPairs[0], newPairs[0]) {
			return []entities.SubgroupDiff{{
				Subgroup: "",
				Type:     entities.ChangeTypeModified,
				OldPair:  oldPairs[0],
				NewPair:  newPairs[0],
			}}
		}
		return nil
	}

	var diffs []entities.SubgroupDiff
	maxLen := max(len(oldPairs), len(newPairs))
	for i := range maxLen {
		var op, np *entities.Pair
		if i < len(oldPairs) {
			op = oldPairs[i]
		}
		if i < len(newPairs) {
			np = newPairs[i]
		}

		switch {
		case op != nil && np != nil:
			if !isSamePair(op, np) {
				diffs = append(diffs, entities.SubgroupDiff{
					Subgroup: "",
					Type:     entities.ChangeTypeModified,
					OldPair:  op,
					NewPair:  np,
				})
			}
		case op != nil:
			diffs = append(diffs, entities.SubgroupDiff{
				Subgroup: "",
				Type:     entities.ChangeTypeRemoved,
				OldPair:  op,
			})
		case np != nil:
			diffs = append(diffs, entities.SubgroupDiff{
				Subgroup: "",
				Type:     entities.ChangeTypeAdded,
				NewPair:  np,
			})
		}
	}
	return diffs
}

func makeTeacherDiffs(pairs []*entities.Pair, changeType entities.ChangeType) []entities.SubgroupDiff {
	var diffs []entities.SubgroupDiff
	for _, p := range pairs {
		diff := entities.SubgroupDiff{
			Subgroup: "",
			Type:     changeType,
		}
		if changeType == entities.ChangeTypeAdded {
			diff.NewPair = p
		} else {
			diff.OldPair = p
		}
		diffs = append(diffs, diff)
	}
	return diffs
}

func maxDayCount(oldDays, newDays []*entities.Day) int {
	m := max(len(oldDays), len(newDays))
	if m > 6 {
		return 6
	}
	return m
}

func groupDays(g *entities.Group) []*entities.Day {
	if g != nil {
		return g.Days
	}
	return nil
}

func teacherDays(t *entities.Teacher) []*entities.Day {
	if t != nil {
		return t.Days
	}
	return nil
}

func getDayName(oldDays, newDays []*entities.Day, dayIdx int) string {
	if dayIdx < len(dayNames) {
		return dayNames[dayIdx]
	}
	if newDays != nil && dayIdx < len(newDays) && newDays[dayIdx] != nil && newDays[dayIdx].Name != "" {
		return cleanDayName(newDays[dayIdx].Name)
	}
	if oldDays != nil && dayIdx < len(oldDays) && oldDays[dayIdx] != nil && oldDays[dayIdx].Name != "" {
		return cleanDayName(oldDays[dayIdx].Name)
	}
	return ""
}

func isSamePair(p1, p2 *entities.Pair) bool {
	if p1 == nil && p2 == nil {
		return true
	}
	if p1 == nil || p2 == nil {
		return false
	}
	return strings.TrimSpace(p1.Name) == strings.TrimSpace(p2.Name) &&
		strings.TrimSpace(p1.Type) == strings.TrimSpace(p2.Type) &&
		strings.TrimSpace(p1.Group) == strings.TrimSpace(p2.Group) &&
		strings.TrimSpace(p1.Class) == strings.TrimSpace(p2.Class)
}

func isPairEmpty(p *entities.Pair) bool {
	if p == nil {
		return true
	}
	name := strings.TrimSpace(p.Name)
	hasName := name != "" && name != "-" && name != "—" && name != "–"
	group := strings.TrimSpace(p.Group)
	return !hasName && group == ""
}

func extractNonEmptyPairs(subj *entities.Subject) []*entities.Pair {
	if subj == nil || subj.IsEmpty {
		return nil
	}
	var res []*entities.Pair
	for _, p := range subj.Pairs {
		if !isPairEmpty(p) {
			res = append(res, p)
		}
	}
	return res
}

func extractSubgroupFromPair(p *entities.Pair) string {
	if p == nil {
		return ""
	}
	if sg := extractSubgroup(p.Group); sg != "" {
		return sg
	}
	name := strings.TrimSpace(p.Name)
	if strings.HasPrefix(name, "1.") || strings.HasPrefix(name, "1. ") {
		return "1 п/г"
	}
	if strings.HasPrefix(name, "2.") || strings.HasPrefix(name, "2. ") {
		return "2 п/г"
	}
	return ""
}

func hasSubgroups(pairs []*entities.Pair) bool {
	for _, p := range pairs {
		if extractSubgroupFromPair(p) != "" {
			return true
		}
	}
	return false
}

func extractSubgroup(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	if strings.Contains(lower, "1 п/г") || strings.Contains(lower, "1 п/гр") || s == "1" {
		return "1 п/г"
	}
	if strings.Contains(lower, "2 п/г") || strings.Contains(lower, "2 п/гр") || s == "2" {
		return "2 п/г"
	}
	if strings.Contains(lower, "п/г") || strings.Contains(lower, "подгруппа") {
		return s
	}
	return ""
}

func cleanDayName(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "|"); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if idx := strings.Index(s, ","); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}
