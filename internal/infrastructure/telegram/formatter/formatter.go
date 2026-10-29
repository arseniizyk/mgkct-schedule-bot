package formatter

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

func FormatErrorMessage(err error) string {
	switch {
	case errors.Is(err, domainerr.ErrUserNoGroup):
		return messages.UserNoGroup
	case errors.Is(err, domainerr.ErrUserNoTeacher):
		return messages.NoTeacherSet
	case errors.Is(err, domainerr.ErrTeacherNotFound):
		return messages.TeacherNotFound
	case errors.Is(err, domainerr.ErrGroupNotFound), errors.Is(err, domainerr.ErrScheduleNotFound):
		return messages.GroupNotFound
	case errors.Is(err, domainerr.ErrServiceInternal):
		return messages.Internal
	default:
		return messages.Internal
	}
}

const secondShiftMinIndex = 3

var (
	minskLoc = time.FixedZone("MSK", 3*3600)
	nowFunc  = time.Now
)

func isCurrentWeek(week, currentWeekMonday time.Time) bool {
	if week.IsZero() {
		return true
	}
	w := week.In(minskLoc)
	weekMonday := time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, minskLoc)
	return weekMonday.Equal(currentWeekMonday)
}

func currentWeekMondayInMinsk(nowMinsk time.Time) time.Time {
	today := (int(nowMinsk.Weekday()) + 6) % 7
	return time.Date(nowMinsk.Year(), nowMinsk.Month(), nowMinsk.Day(), 0, 0, 0, 0, minskLoc).AddDate(0, 0, -today)
}

func isDayToday(week time.Time, dayIdx int) bool {
	nowMinsk := nowFunc().In(minskLoc)
	todayIdx := (int(nowMinsk.Weekday()) + 6) % 7
	if todayIdx != dayIdx {
		return false
	}
	currentMonday := currentWeekMondayInMinsk(nowMinsk)
	return isCurrentWeek(week, currentMonday)
}

func formatScheduleDay(day *entities.Day, isToday bool) string {
	var sb strings.Builder
	sb.Grow(256)

	first := findFirstSubject(day.Subjects)

	skip := 0
	isSecondShift := first >= secondShiftMinIndex
	if isSecondShift {
		skip = first
	}

	var titleParts []string
	titleParts = append(titleParts, escapeMD(day.Name))
	if isToday {
		titleParts = append(titleParts, messages.Today)
	}
	if isSecondShift {
		titleParts = append(titleParts, messages.SecondShift)
	}

	fmt.Fprintf(&sb, "*%s\n*", strings.Join(titleParts, " "))

	sb.WriteString(formatSubjects(day.Subjects, skip))
	sb.WriteString("\n")

	return sb.String()
}

func findFirstSubject(subjects []*entities.Subject) int {
	for i, subject := range subjects {
		if !subject.IsEmpty {
			return i
		}
	}
	return -1
}

func EffectiveDayIndex(group *entities.Group) int {
	if group == nil || len(group.Days) == 0 {
		return 0
	}

	nowMinsk := time.Now().In(minskLoc)
	today := (int(nowMinsk.Weekday()) + 6) % 7 // Пн=0 .. Вс=6
	offset := today
	currentWeekMonday := time.Date(nowMinsk.Year(), nowMinsk.Month(), nowMinsk.Day(), 0, 0, 0, 0, minskLoc).AddDate(0, 0, -offset)

	// Если расписание относится к будущей или прошедшей неделе (относительно текущей), показываем первый учебный день (понедельник)
	if !isCurrentWeek(group.Week, currentWeekMonday) {
		return 0
	}

	if today >= len(group.Days) {
		return 0
	}

	lastSubject := findLastSubject(group.Days[today].Subjects)
	if lastSubject == -1 {
		return nextWorkDayIndex(today)
	}

	if endTime, ok := getEndTime(today, lastSubject+1); ok && !nowMinsk.Before(endTime) {
		return nextWorkDayIndex(today)
	}

	return today
}

func nextWorkDayIndex(idx int) int {
	next := idx + 1
	if next >= 6 {
		next = 0
	}
	return next
}

// ParsePairSubgroup определяет номер подгруппы (1, 2, 3...) из пары.
// Возвращает 0, если пара общая для всей группы или подгруппа не определена.
func ParsePairSubgroup(p *entities.Pair) int {
	if p == nil {
		return 0
	}
	name := strings.TrimSpace(p.Name)
	if len(name) >= 2 && unicode.IsDigit(rune(name[0])) && name[1] == '.' {
		return int(name[0] - '0')
	}

	lower := strings.ToLower(p.Group)
	if strings.Contains(lower, "1 п/г") || strings.Contains(lower, "1 п/гр") || strings.TrimSpace(p.Group) == "1" {
		return 1
	}
	if strings.Contains(lower, "2 п/г") || strings.Contains(lower, "2 п/гр") || strings.TrimSpace(p.Group) == "2" {
		return 2
	}
	if strings.Contains(lower, "3 п/г") || strings.Contains(lower, "3 п/гр") || strings.TrimSpace(p.Group) == "3" {
		return 3
	}

	return 0
}

// FilterGroupForSubgroup возвращает копию расписания группы, отфильтрованную для указанной подгруппы:
// - Если subgroup <= 0, возвращается группа без изменений.
// - Для пар с 2 подгруппами пользователю выводится только его расписание.
// - Для пар с 3 подгруппами делается исключение: выводится расписание для всех.
// - Общие пары без деления на подгруппы выводятся всем.
func FilterGroupForSubgroup(group *entities.Group, subgroup int) *entities.Group {
	if group == nil || subgroup <= 0 {
		return group
	}

	filteredGroup := &entities.Group{
		ID:   group.ID,
		Week: group.Week,
		Days: make([]*entities.Day, len(group.Days)),
	}

	for dIdx, day := range group.Days {
		if day == nil {
			continue
		}

		filteredDay := &entities.Day{
			Name:     day.Name,
			Subjects: make([]*entities.Subject, len(day.Subjects)),
		}

		for sIdx, subj := range day.Subjects {
			if subj == nil || subj.IsEmpty || len(subj.Pairs) == 0 {
				filteredDay.Subjects[sIdx] = &entities.Subject{IsEmpty: true}
				continue
			}

			var (
				nonEmptyPairs []*entities.Pair
				subgroupsSet  = make(map[int]struct{})
				hasSubgroups  = false
			)

			for _, p := range subj.Pairs {
				if p == nil {
					continue
				}
				name := strings.TrimSpace(p.Name)
				if name == "" && p.Group == "" {
					continue
				}
				nonEmptyPairs = append(nonEmptyPairs, p)
				sg := ParsePairSubgroup(p)
				if sg > 0 {
					hasSubgroups = true
					subgroupsSet[sg] = struct{}{}
				}
			}

			if len(nonEmptyPairs) == 0 {
				filteredDay.Subjects[sIdx] = &entities.Subject{IsEmpty: true}
				continue
			}

			// Для пар с 3 подгруппами исключение: выводим расписание для всех
			_, hasSg3 := subgroupsSet[3]
			if hasSg3 || len(subgroupsSet) >= 3 {
				filteredDay.Subjects[sIdx] = subj
				continue
			}

			// Для пар с 2 подгруппами выводим только расписание указанной подгруппы
			if hasSubgroups {
				var userPairs []*entities.Pair
				for _, p := range nonEmptyPairs {
					sg := ParsePairSubgroup(p)
					if sg == subgroup || sg == 0 {
						userPairs = append(userPairs, p)
					}
				}

				if len(userPairs) == 0 {
					filteredDay.Subjects[sIdx] = &entities.Subject{IsEmpty: true}
				} else {
					filteredDay.Subjects[sIdx] = &entities.Subject{
						Pairs:   userPairs,
						IsEmpty: pairsAreEmpty(userPairs),
					}
				}
				continue
			}

			// Общая пара без подгрупп
			filteredDay.Subjects[sIdx] = subj
		}

		filteredGroup.Days[dIdx] = filteredDay
	}

	return filteredGroup
}

func FormatScheduleDay(group *entities.Group) string {
	return FormatScheduleDayForSubgroup(group, 0)
}

func FormatScheduleDayForSubgroup(group *entities.Group, subgroup int) string {
	filtered := FilterGroupForSubgroup(group, subgroup)
	dayIdx := EffectiveDayIndex(filtered)
	if dayIdx >= len(filtered.Days) {
		return formatScheduleDay(filtered.Days[0], isDayToday(group.Week, 0))
	}
	return formatScheduleDay(filtered.Days[dayIdx], isDayToday(group.Week, dayIdx))
}

func FormatScheduleDayAt(group *entities.Group, dayIdx int) (string, error) {
	return FormatScheduleDayAtForSubgroup(group, dayIdx, 0)
}

func FormatScheduleDayAtForSubgroup(group *entities.Group, dayIdx int, subgroup int) (string, error) {
	filtered := FilterGroupForSubgroup(group, subgroup)
	if dayIdx < 0 || dayIdx >= len(filtered.Days) {
		return "", fmt.Errorf("formatter: day index %d out of range (%d days)", dayIdx, len(filtered.Days))
	}
	return formatScheduleDay(filtered.Days[dayIdx], isDayToday(group.Week, dayIdx)), nil
}

var weekdaysTimeEnd = map[int][2]int{
	1: {10, 40},
	2: {12, 40},
	3: {14, 40},
	4: {16, 30},
	5: {18, 20},
	6: {20, 10},
}

var weekendTimeEnd = map[int][2]int{
	1: {10, 40},
	2: {12, 40},
	3: {14, 30},
	4: {16, 20},
	5: {18, 10},
	6: {20, 0},
}

// GetDayEndTime возвращает время окончания пар для указанного дня (0=Пн..5=Сб) и номера последней пары (1-based).
func GetDayEndTime(dayIdx, lastSubject int, loc *time.Location) (time.Time, bool) {
	var hhmm [2]int
	var ok bool

	if dayIdx == 5 {
		hhmm, ok = weekendTimeEnd[lastSubject]
	} else {
		hhmm, ok = weekdaysTimeEnd[lastSubject]
	}

	if !ok {
		return time.Time{}, false
	}

	if loc == nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), hhmm[0], hhmm[1], 0, 0, loc)
	return end, true
}

func getEndTime(dayIdx, lastSubject int) (time.Time, bool) {
	return GetDayEndTime(dayIdx, lastSubject, minskLoc)
}

func FormatScheduleWeek(group *entities.Group) string {
	return FormatScheduleWeekForSubgroup(group, 0)
}

func FormatScheduleWeekForSubgroup(group *entities.Group, subgroup int) string {
	filtered := FilterGroupForSubgroup(group, subgroup)
	var sb strings.Builder
	sb.Grow(len(filtered.Days) * 128)

	for i, day := range filtered.Days {
		sb.WriteString(formatScheduleDay(day, isDayToday(group.Week, i)))
	}

	return sb.String()
}

const maxMessageLength = 4096

func SplitMessage(text string, limit int) []string {
	if limit <= 0 {
		limit = maxMessageLength
	}

	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}

	var chunks []string
	var current strings.Builder

	for line := range strings.SplitSeq(text, "\n") {
		lineRunes := []rune(line)
		if current.Len() > 0 && len([]rune(current.String()))+len(lineRunes)+1 > limit {
			chunks = append(chunks, current.String())
			current.Reset()
		}

		if len(lineRunes) > limit {
			for len(lineRunes) > 0 {
				take := min(limit, len(lineRunes))
				chunks = append(chunks, string(lineRunes[:take]))
				lineRunes = lineRunes[take:]
			}
			continue
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

func pairsAreEmpty(pairs []*entities.Pair) bool {
	for _, p := range pairs {
		if p == nil {
			continue
		}
		name := strings.TrimSpace(p.Name)
		hasContent := (name != "" && name != "-" && name != "—" && name != "–") || strings.TrimSpace(p.Group) != ""
		if hasContent {
			return false
		}
	}
	return true
}

func formatSubjects(subjects []*entities.Subject, start int) string {
	var sb strings.Builder
	sb.Grow(len(subjects) * 80)

	lastSubject := findLastSubject(subjects)
	if lastSubject == -1 {
		return "*Выходной*\n"
	}

	for i := start; i < len(subjects); i++ {
		subject := subjects[i]

		if subjectHasNoClasses(subject) {
			if i > lastSubject {
				break
			}
			fmt.Fprintf(&sb, "%d: ──\n", i+1)
			continue
		}

		pairs := subject.Pairs
		if len(pairs) == 1 && len(pairs[0].Name) > 0 && !unicode.IsDigit(rune(pairs[0].Name[0])) {
			p := pairs[0]
			fmt.Fprintf(&sb, "%d: %s | %s | %s", i+1, escapeMD(p.Name), escapeMD(p.Type), escapeMD(p.Group))
			sb.WriteString(formatClass(p.Class))
			sb.WriteString("\n")
			continue
		}

		fmt.Fprintf(&sb, "%d:\n", i+1)
		for j, p := range pairs {
			if j == len(pairs)-1 {
				sb.WriteString("└─ ")
			} else {
				sb.WriteString("├─ ")
			}
			fmt.Fprintf(&sb, "%s | %s | %s", escapeMD(p.Name), escapeMD(p.Type), escapeMD(p.Group))
			sb.WriteString(formatClass(p.Class))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatClass(class string) string {
	if class != "-" && class != "" {
		return " | " + escapeMD(class)
	}
	return ""
}

var mdEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"_", "\\_",
	"*", "\\*",
	"[", "\\[",
	"`", "\\`",
)

func escapeMD(s string) string {
	return mdEscaper.Replace(s)
}

// FindLastSubject находит индекс последнего непустого предмета в дне (0-based) или -1 если пар нет.
func FindLastSubject(subjects []*entities.Subject) int {
	return findLastSubject(subjects)
}

func findLastSubject(subjects []*entities.Subject) int {
	if len(subjects) == 0 {
		return -1
	}

	for i, subject := range slices.Backward(subjects) {
		if !subjectHasNoClasses(subject) {
			return i
		}
	}

	return -1
}

func subjectHasNoClasses(subject *entities.Subject) bool {
	if subject == nil || subject.IsEmpty {
		return true
	}

	for _, p := range subject.Pairs {
		if p.Name != "" || p.Group != "" {
			return false
		}
	}

	return true
}

func FormatTeacherProfile(member *entities.FacultyMember) string {
	if member == nil {
		return ""
	}
	var sb strings.Builder
	if member.FullName != "" && member.FullName != member.ShortName {
		fmt.Fprintf(&sb, "👨‍🏫 *%s* (%s)\n", escapeMD(member.FullName), escapeMD(member.ShortName))
	}
	var meta []string
	if member.Position != "" {
		meta = append(meta, member.Position)
	}
	if member.Qualification != "" {
		meta = append(meta, member.Qualification)
	}
	if len(meta) > 0 {
		fmt.Fprintf(&sb, "_%s_\n", escapeMD(strings.Join(meta, " • ")))
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}

func FormatTeacherScheduleDay(teacher *entities.Teacher) string {
	dayIdx := EffectiveTeacherDayIndex(teacher)
	if dayIdx >= len(teacher.Days) {
		return formatTeacherScheduleDay(teacher.Days[0], isDayToday(teacher.Week, 0))
	}
	return formatTeacherScheduleDay(teacher.Days[dayIdx], isDayToday(teacher.Week, dayIdx))
}

func FormatTeacherScheduleDayAt(teacher *entities.Teacher, dayIdx int) (string, error) {
	if dayIdx < 0 || dayIdx >= len(teacher.Days) {
		return "", fmt.Errorf("formatter: day index %d out of range (%d days)", dayIdx, len(teacher.Days))
	}
	return formatTeacherScheduleDay(teacher.Days[dayIdx], isDayToday(teacher.Week, dayIdx)), nil
}

func formatTeacherScheduleDay(day *entities.Day, isToday bool) string {
	var sb strings.Builder
	sb.Grow(256)

	first := findFirstSubject(day.Subjects)

	skip := 0
	isSecondShift := first >= secondShiftMinIndex
	if isSecondShift {
		skip = first
	}

	var titleParts []string
	titleParts = append(titleParts, escapeMD(day.Name))
	if isToday {
		titleParts = append(titleParts, messages.Today)
	}
	if isSecondShift {
		titleParts = append(titleParts, messages.SecondShift)
	}

	fmt.Fprintf(&sb, "*%s\n*", strings.Join(titleParts, " "))

	sb.WriteString(formatTeacherSubjects(day.Subjects, skip))
	sb.WriteString("\n")

	return sb.String()
}

func EffectiveTeacherDayIndex(teacher *entities.Teacher) int {
	if teacher == nil || len(teacher.Days) == 0 {
		return 0
	}

	nowMinsk := time.Now().In(minskLoc)
	today := (int(nowMinsk.Weekday()) + 6) % 7
	offset := today
	currentWeekMonday := time.Date(nowMinsk.Year(), nowMinsk.Month(), nowMinsk.Day(), 0, 0, 0, 0, minskLoc).AddDate(0, 0, -offset)

	// Если расписание относится к будущей или прошедшей неделе (относительно текущей), показываем первый учебный день (понедельник)
	if !isCurrentWeek(teacher.Week, currentWeekMonday) {
		return 0
	}

	if today >= len(teacher.Days) {
		return 0
	}

	lastSubject := findLastSubject(teacher.Days[today].Subjects)
	if lastSubject == -1 {
		return nextWorkDayIndex(today)
	}

	if endTime, ok := getEndTime(today, lastSubject+1); ok && !nowMinsk.Before(endTime) {
		return nextWorkDayIndex(today)
	}

	return today
}

func FormatTeacherScheduleWeek(teacher *entities.Teacher) string {
	var sb strings.Builder
	sb.Grow(len(teacher.Days) * 128)

	for i, day := range teacher.Days {
		sb.WriteString(formatTeacherScheduleDay(day, isDayToday(teacher.Week, i)))
	}

	return sb.String()
}

func formatTeacherSubjects(subjects []*entities.Subject, start int) string {
	var sb strings.Builder
	sb.Grow(len(subjects) * 80)

	lastSubject := findLastSubject(subjects)
	if lastSubject == -1 {
		return "*Выходной*\n"
	}

	for i := start; i < len(subjects); i++ {
		subject := subjects[i]

		if subjectHasNoClasses(subject) {
			if i > lastSubject {
				break
			}
			fmt.Fprintf(&sb, "%d: ──\n", i+1)
			continue
		}

		pairs := subject.Pairs
		if len(pairs) == 1 && len(pairs[0].Name) > 0 && !unicode.IsDigit(rune(pairs[0].Name[0])) {
			p := pairs[0]
			fmt.Fprintf(&sb, "%d: %s | %s | %s", i+1, escapeMD(p.Name), escapeMD(p.Type), escapeMD(p.Group))
			sb.WriteString(formatClass(p.Class))
			sb.WriteString("\n")
			continue
		}

		fmt.Fprintf(&sb, "%d:\n", i+1)
		for j, p := range pairs {
			if j == len(pairs)-1 {
				sb.WriteString("└─ ")
			} else {
				sb.WriteString("├─ ")
			}
			fmt.Fprintf(&sb, "%s | %s | %s", escapeMD(p.Name), escapeMD(p.Type), escapeMD(p.Group))
			sb.WriteString(formatClass(p.Class))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
