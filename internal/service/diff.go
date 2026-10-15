package service

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

var (
	minskLoc = time.FixedZone("MSK", 3*3600)

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

// CurrentWeekMonday возвращает полночь понедельника (UTC+3 / Europe/Minsk) для недели, содержащей указанное время.
func CurrentWeekMonday(now time.Time) time.Time {
	localNow := now.In(minskLoc)
	weekday := localNow.Weekday()
	offset := (int(weekday) + 6) % 7 // Пн=0, Вт=1, ..., Вс=6
	return time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, minskLoc).AddDate(0, 0, -offset)
}

// ComputeGroupDiff вычисляет точечные изменения в расписании учебной группы с учетом фильтрации прошедшего времени.
func ComputeGroupDiff(oldGroup, newGroup *entities.Group, now time.Time) *entities.ScheduleDiff {
	if newGroup == nil && oldGroup == nil {
		return nil
	}

	if now.IsZero() {
		now = time.Now()
	}
	nowMinsk := now.In(minskLoc)
	currentWeekMonday := CurrentWeekMonday(nowMinsk)

	var targetWeek time.Time
	var groupID int32
	if newGroup != nil {
		targetWeek = newGroup.Week
		groupID = newGroup.ID
	} else {
		targetWeek = oldGroup.Week
		groupID = oldGroup.ID
	}

	weekMonday := time.Date(targetWeek.Year(), targetWeek.Month(), targetWeek.Day(), 0, 0, 0, 0, minskLoc)
	if weekMonday.Before(currentWeekMonday) {
		return nil
	}

	isCurrentWeek := weekMonday.Equal(currentWeekMonday)
	todayDate := time.Date(nowMinsk.Year(), nowMinsk.Month(), nowMinsk.Day(), 0, 0, 0, 0, minskLoc)

	maxDays := maxDayCount(oldGroupDays(oldGroup), newGroupDays(newGroup))
	var daysDiff []entities.DayDiff

	for dayIdx := range maxDays {
		dayDate := weekMonday.AddDate(0, 0, dayIdx)
		if isCurrentWeek && dayDate.Before(todayDate) {
			continue
		}

		dayDiff := computeDayGroupDiff(oldGroup, newGroup, dayIdx, dayDate, todayDate, isCurrentWeek, nowMinsk)
		if dayDiff != nil {
			daysDiff = append(daysDiff, *dayDiff)
		}
	}

	if len(daysDiff) == 0 {
		return nil
	}

	return &entities.ScheduleDiff{
		Title: fmt.Sprintf("Группа %d", groupID),
		Week:  weekMonday,
		Days:  daysDiff,
	}
}

func computeDayGroupDiff(
	oldGroup, newGroup *entities.Group,
	dayIdx int,
	dayDate, todayDate time.Time,
	isCurrentWeek bool,
	nowMinsk time.Time,
) *entities.DayDiff {
	dayName := getDayName(oldGroupDays(oldGroup), newGroupDays(newGroup), dayIdx)

	var oldSubjects, newSubjects []*entities.Subject
	if oldGroup != nil && dayIdx < len(oldGroup.Days) {
		oldSubjects = oldGroup.Days[dayIdx].Subjects
	}
	if newGroup != nil && dayIdx < len(newGroup.Days) {
		newSubjects = newGroup.Days[dayIdx].Subjects
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

		sgDiffs := compareGroupPairs(oldPairs, newPairs)
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

// ComputeTeacherDiff вычисляет точечные изменения в расписании преподавателя с учетом фильтрации прошедшего времени.
func ComputeTeacherDiff(oldTeacher, newTeacher *entities.Teacher, now time.Time) *entities.ScheduleDiff {
	if newTeacher == nil && oldTeacher == nil {
		return nil
	}

	if now.IsZero() {
		now = time.Now()
	}
	nowMinsk := now.In(minskLoc)
	currentWeekMonday := CurrentWeekMonday(nowMinsk)

	var targetWeek time.Time
	var teacherName string
	if newTeacher != nil {
		targetWeek = newTeacher.Week
		teacherName = newTeacher.Name
	} else {
		targetWeek = oldTeacher.Week
		teacherName = oldTeacher.Name
	}

	weekMonday := time.Date(targetWeek.Year(), targetWeek.Month(), targetWeek.Day(), 0, 0, 0, 0, minskLoc)
	if weekMonday.Before(currentWeekMonday) {
		return nil
	}

	isCurrentWeek := weekMonday.Equal(currentWeekMonday)
	todayDate := time.Date(nowMinsk.Year(), nowMinsk.Month(), nowMinsk.Day(), 0, 0, 0, 0, minskLoc)

	maxDays := maxDayCount(oldTeacherDays(oldTeacher), newTeacherDays(newTeacher))
	var daysDiff []entities.DayDiff

	for dayIdx := range maxDays {
		dayDate := weekMonday.AddDate(0, 0, dayIdx)
		if isCurrentWeek && dayDate.Before(todayDate) {
			continue
		}

		dayDiff := computeDayTeacherDiff(oldTeacher, newTeacher, dayIdx, dayDate, todayDate, isCurrentWeek, nowMinsk)
		if dayDiff != nil {
			daysDiff = append(daysDiff, *dayDiff)
		}
	}

	if len(daysDiff) == 0 {
		return nil
	}

	return &entities.ScheduleDiff{
		Title: teacherName,
		Week:  weekMonday,
		Days:  daysDiff,
	}
}

func computeDayTeacherDiff(
	oldTeacher, newTeacher *entities.Teacher,
	dayIdx int,
	dayDate, todayDate time.Time,
	isCurrentWeek bool,
	nowMinsk time.Time,
) *entities.DayDiff {
	dayName := getDayName(oldTeacherDays(oldTeacher), newTeacherDays(newTeacher), dayIdx)

	var oldSubjects, newSubjects []*entities.Subject
	if oldTeacher != nil && dayIdx < len(oldTeacher.Days) {
		oldSubjects = oldTeacher.Days[dayIdx].Subjects
	}
	if newTeacher != nil && dayIdx < len(newTeacher.Days) {
		newSubjects = newTeacher.Days[dayIdx].Subjects
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

		sgDiffs := compareTeacherPairs(oldPairs, newPairs)
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
		return compareGeneralPairs(oldPairs, newPairs)
	}
}

func makeAllAddedDiffs(pairs []*entities.Pair) []entities.SubgroupDiff {
	var diffs []entities.SubgroupDiff
	for _, np := range pairs {
		diffs = append(diffs, entities.SubgroupDiff{
			Subgroup: extractSubgroup(np.Group),
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
			Subgroup: extractSubgroup(op.Group),
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
			Subgroup: extractSubgroup(op.Group),
			Type:     entities.ChangeTypeRemoved,
			OldPair:  op,
		})
	}
	for _, np := range newPairs {
		diffs = append(diffs, entities.SubgroupDiff{
			Subgroup: extractSubgroup(np.Group),
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
		sg := extractSubgroup(p.Group)
		oldBySg[sg] = p
		if !slices.Contains(sgKeys, sg) {
			sgKeys = append(sgKeys, sg)
		}
	}
	for _, p := range newPairs {
		sg := extractSubgroup(p.Group)
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

func compareGeneralPairs(oldPairs, newPairs []*entities.Pair) []entities.SubgroupDiff {
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

func oldGroupDays(g *entities.Group) []*entities.Day {
	if g != nil {
		return g.Days
	}
	return nil
}

func newGroupDays(g *entities.Group) []*entities.Day {
	if g != nil {
		return g.Days
	}
	return nil
}

func oldTeacherDays(t *entities.Teacher) []*entities.Day {
	if t != nil {
		return t.Days
	}
	return nil
}

func newTeacherDays(t *entities.Teacher) []*entities.Day {
	if t != nil {
		return t.Days
	}
	return nil
}

func getDayName(oldDays, newDays []*entities.Day, dayIdx int) string {
	if dayIdx < len(dayNames) {
		return dayNames[dayIdx]
	}
	if newDays != nil && dayIdx < len(newDays) && newDays[dayIdx].Name != "" {
		return cleanDayName(newDays[dayIdx].Name)
	}
	if oldDays != nil && dayIdx < len(oldDays) && oldDays[dayIdx].Name != "" {
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

func hasSubgroups(pairs []*entities.Pair) bool {
	for _, p := range pairs {
		if extractSubgroup(p.Group) != "" {
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
