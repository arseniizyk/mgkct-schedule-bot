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

func formatScheduleDay(day *entities.Day) string {
	var sb strings.Builder
	sb.Grow(256)

	first := findFirstSubject(day.Subjects)

	skip := 0
	if first >= secondShiftMinIndex {
		fmt.Fprintf(&sb, "*%s %s\n*", escapeMD(day.Name), messages.SecondShift)
		skip = first
	} else {
		fmt.Fprintf(&sb, "*%s\n*", escapeMD(day.Name))
	}

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
	today := (int(time.Now().Weekday()) + 6) % 7 // Пн=0 .. Вс=6

	if today >= len(group.Days) {
		return 0
	}

	lastSubject := findLastSubject(group.Days[today].Subjects)
	if lastSubject == -1 {
		return nextWorkDayIndex(today)
	}

	if endTime, ok := getEndTime(today, lastSubject+1); ok && !time.Now().Before(endTime) {
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

func FormatScheduleDay(group *entities.Group) string {
	dayIdx := EffectiveDayIndex(group)
	if dayIdx >= len(group.Days) {
		return formatScheduleDay(group.Days[0])
	}
	return formatScheduleDay(group.Days[dayIdx])
}

func FormatScheduleDayAt(group *entities.Group, dayIdx int) (string, error) {
	if dayIdx < 0 || dayIdx >= len(group.Days) {
		return "", fmt.Errorf("formatter: day index %d out of range (%d days)", dayIdx, len(group.Days))
	}
	return formatScheduleDay(group.Days[dayIdx]), nil
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

func getEndTime(dayIdx, lastSubject int) (time.Time, bool) {
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

	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), hhmm[0], hhmm[1], 0, 0, time.Local)
	return end, true
}

func FormatScheduleWeek(group *entities.Group) string {
	var sb strings.Builder
	sb.Grow(len(group.Days) * 128)

	for _, day := range group.Days {
		sb.WriteString(formatScheduleDay(day))
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
		return formatTeacherScheduleDay(teacher.Days[0])
	}
	return formatTeacherScheduleDay(teacher.Days[dayIdx])
}

func FormatTeacherScheduleDayAt(teacher *entities.Teacher, dayIdx int) (string, error) {
	if dayIdx < 0 || dayIdx >= len(teacher.Days) {
		return "", fmt.Errorf("formatter: day index %d out of range (%d days)", dayIdx, len(teacher.Days))
	}
	return formatTeacherScheduleDay(teacher.Days[dayIdx]), nil
}

func formatTeacherScheduleDay(day *entities.Day) string {
	var sb strings.Builder
	sb.Grow(256)

	first := findFirstSubject(day.Subjects)

	skip := 0
	if first >= secondShiftMinIndex {
		fmt.Fprintf(&sb, "*%s %s\n*", escapeMD(day.Name), messages.SecondShift)
		skip = first
	} else {
		fmt.Fprintf(&sb, "*%s\n*", escapeMD(day.Name))
	}

	sb.WriteString(formatTeacherSubjects(day.Subjects, skip))
	sb.WriteString("\n")

	return sb.String()
}

func EffectiveTeacherDayIndex(teacher *entities.Teacher) int {
	today := (int(time.Now().Weekday()) + 6) % 7
	if today >= len(teacher.Days) {
		return 0
	}

	lastSubject := findLastSubject(teacher.Days[today].Subjects)
	if lastSubject == -1 {
		return nextWorkDayIndex(today)
	}

	if endTime, ok := getEndTime(today, lastSubject+1); ok && !time.Now().Before(endTime) {
		return nextWorkDayIndex(today)
	}

	return today
}

func FormatTeacherScheduleWeek(teacher *entities.Teacher) string {
	var sb strings.Builder
	sb.Grow(len(teacher.Days) * 128)

	for _, day := range teacher.Days {
		sb.WriteString(formatTeacherScheduleDay(day))
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
