package formatter

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
)

func FormatErrorMessage(err error) string {
	switch {
	case errors.Is(err, domainerr.ErrUserNoGroup):
		return messages.UserNoGroup

	case errors.Is(err, domainerr.ErrUserNoTeacher):
		return messages.NoTeacherSet

	case errors.Is(err, domainerr.ErrTeacherNotFound):
		return messages.TeacherNotFound

	case errors.Is(err, domainerr.ErrGroupNotFound):
		return messages.GroupNotFound

	case errors.Is(err, domainerr.ErrScheduleNotFound):
		return messages.GroupNotFound

	case errors.Is(err, domainerr.ErrServiceInternal):
		return messages.Internal

	default:
		return messages.Internal
	}
}

// secondShiftMinIndex — минимальный индекс непустой пары, начиная с которого
// группа считается обучающейся во вторую смену (пары №4 и далее).
const secondShiftMinIndex = 3

func formatScheduleDay(day *pb.Day) string {
	var sb strings.Builder
	sb.Grow(256)

	first := findFirstSubject(day.GetSubjects())

	skip := 0
	if first >= secondShiftMinIndex {
		fmt.Fprintf(&sb, "*%s %s\n*", escapeMD(day.GetName()), messages.SecondShift)
		skip = first // чужие слоты первой смены не показываем
	} else {
		fmt.Fprintf(&sb, "*%s\n*", escapeMD(day.GetName()))
	}

	sb.WriteString(formatSubjects(day.GetSubjects(), skip))
	sb.WriteString("\n")

	return sb.String()
}

func findFirstSubject(subjects []*pb.Subject) int {
	for i, subject := range subjects {
		if !subject.GetIsEmpty() {
			return i
		}
	}

	return -1
}

// EffectiveDayIndex возвращает индекс дня, который показываем по умолчанию:
// сегодня, а если пары закончились или день пуст — следующий рабочий день.
func EffectiveDayIndex(group *pb.Group) int {
	today := (int(time.Now().Weekday()) + 6) % 7 // Пн=0 .. Вс=6

	if today >= len(group.GetDays()) {
		return 0
	}

	lastSubject := findLastSubject(group.GetDays()[today].GetSubjects())
	if lastSubject == -1 {
		return nextWorkDayIndex(today)
	}

	if endTime, ok := getEndTime(today, lastSubject+1); ok && !time.Now().Before(endTime) {
		return nextWorkDayIndex(today)
	}

	return today
}

// воскресенья в расписании нет, за субботой идёт понедельник
func nextWorkDayIndex(idx int) int {
	next := idx + 1
	if next >= 6 {
		next = 0
	}
	return next
}

// FormatScheduleDay форматирует день по умолчанию (см. EffectiveDayIndex).
func FormatScheduleDay(group *pb.Group) string {
	dayIdx := EffectiveDayIndex(group)

	if dayIdx >= len(group.GetDays()) {
		return formatScheduleDay(group.GetDays()[0])
	}

	return formatScheduleDay(group.GetDays()[dayIdx])
}

// FormatScheduleDayAt форматирует конкретный день недели (0 — понедельник).
func FormatScheduleDayAt(group *pb.Group, dayIdx int) (string, error) {
	if dayIdx < 0 || dayIdx >= len(group.GetDays()) {
		return "", fmt.Errorf("formatter: day index %d out of range (%d days)", dayIdx, len(group.GetDays()))
	}

	return formatScheduleDay(group.GetDays()[dayIdx]), nil
}

//nolint:gochecknoglobals // статичные таблицы времени окончания пар
var weekdaysTimeEnd = map[int][2]int{ // map[subjectIndex][hours, min]
	1: {10, 40},
	2: {12, 40},
	3: {14, 40},
	4: {16, 30},
	5: {18, 20},
	6: {20, 10},
}

//nolint:gochecknoglobals // статичная таблица времени окончания пар в субботу
var weekendTimeEnd = map[int][2]int{ // map[subjectIndex][hours, min]
	1: {10, 40},
	2: {12, 40},
	3: {14, 30},
	4: {16, 20},
	5: {18, 10},
	6: {20, 0o0},
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

func FormatScheduleWeek(group *pb.Group) string {
	var sb strings.Builder
	sb.Grow(len(group.GetDays()) * 128)

	for _, day := range group.GetDays() {
		sb.WriteString(formatScheduleDay(day))
	}

	return sb.String()
}

// maxMessageLength — жёсткий лимит Telegram на длину одного сообщения.
const maxMessageLength = 4096

// SplitMessage разбивает длинный текст на части не длиннее limit рун,
// предпочитая разрыв по границам строк. Одиночные слишком длинные строки
// разбиваются принудительно.
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

func formatSubjects(subjects []*pb.Subject, start int) string {
	var sb strings.Builder
	sb.Grow(len(subjects) * 80)

	lastSubject := findLastSubject(subjects)
	if lastSubject == -1 { // if no pairs in day
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

		pairs := subject.GetPairs()
		if len(pairs) == 1 && len(pairs[0].GetName()) > 0 && !unicode.IsDigit(rune(pairs[0].GetName()[0])) { // If only 1 pair in subject and starts with digit
			p := pairs[0]
			fmt.Fprintf(&sb, "%d: %s | %s | %s", i+1, escapeMD(p.GetName()), escapeMD(p.GetType()), escapeMD(p.GetGroup()))
			sb.WriteString(formatClass(p.GetClass()))
			sb.WriteString("\n")
			continue
		}

		fmt.Fprintf(&sb, "%d:\n", i+1)
		for j, p := range pairs {
			if j == len(pairs)-1 { // if last pair in subject
				sb.WriteString("└─ ")
			} else {
				sb.WriteString("├─ ")
			}
			fmt.Fprintf(&sb, "%s | %s | %s", escapeMD(p.GetName()), escapeMD(p.GetType()), escapeMD(p.GetGroup()))
			sb.WriteString(formatClass(p.GetClass()))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatClass(class string) string {
	if class != "-" {
		return " | " + escapeMD(class)
	}
	return ""
}

//nolint:gochecknoglobals // экранирование — константная таблица замен
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

func findLastSubject(subjects []*pb.Subject) int {
	if len(subjects) == 0 || subjects == nil {
		return -1
	}

	for i, subject := range slices.Backward(subjects) {
		if !subjectHasNoClasses(subject) {
			return i
		}
	}

	return -1
}

func subjectHasNoClasses(subject *pb.Subject) bool {
	if subject.GetIsEmpty() {
		return true
	}

	for _, p := range subject.GetPairs() {
		if p.GetName() != "" || p.GetGroup() != "" {
			return false
		}
	}

	return true
}

// FormatTeacherScheduleDay форматирует день расписания преподавателя по умолчанию.
func FormatTeacherScheduleDay(teacher *pb.Teacher) string {
	dayIdx := EffectiveTeacherDayIndex(teacher)

	if dayIdx >= len(teacher.GetDays()) {
		return formatTeacherScheduleDay(teacher.GetDays()[0])
	}

	return formatTeacherScheduleDay(teacher.GetDays()[dayIdx])
}

// FormatTeacherScheduleDayAt форматирует конкретный день недели преподавателя (0 — понедельник).
func FormatTeacherScheduleDayAt(teacher *pb.Teacher, dayIdx int) (string, error) {
	if dayIdx < 0 || dayIdx >= len(teacher.GetDays()) {
		return "", fmt.Errorf("formatter: day index %d out of range (%d days)", dayIdx, len(teacher.GetDays()))
	}

	return formatTeacherScheduleDay(teacher.GetDays()[dayIdx]), nil
}

func formatTeacherScheduleDay(day *pb.Day) string {
	var sb strings.Builder
	sb.Grow(256)

	first := findFirstSubject(day.GetSubjects())

	skip := 0
	if first >= secondShiftMinIndex {
		fmt.Fprintf(&sb, "*%s %s\n*", escapeMD(day.GetName()), messages.SecondShift)
		skip = first
	} else {
		fmt.Fprintf(&sb, "*%s\n*", escapeMD(day.GetName()))
	}

	sb.WriteString(formatTeacherSubjects(day.GetSubjects(), skip))
	sb.WriteString("\n")

	return sb.String()
}

// EffectiveTeacherDayIndex возвращает индекс дня для расписания преподавателя.
func EffectiveTeacherDayIndex(teacher *pb.Teacher) int {
	today := (int(time.Now().Weekday()) + 6) % 7

	if today >= len(teacher.GetDays()) {
		return 0
	}

	lastSubject := findLastSubject(teacher.GetDays()[today].GetSubjects())
	if lastSubject == -1 {
		return nextWorkDayIndex(today)
	}

	if endTime, ok := getEndTime(today, lastSubject+1); ok && !time.Now().Before(endTime) {
		return nextWorkDayIndex(today)
	}

	return today
}

func FormatTeacherScheduleWeek(teacher *pb.Teacher) string {
	var sb strings.Builder
	sb.Grow(len(teacher.GetDays()) * 128)

	for _, day := range teacher.GetDays() {
		sb.WriteString(formatTeacherScheduleDay(day))
	}

	return sb.String()
}

func formatTeacherSubjects(subjects []*pb.Subject, start int) string {
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

		pairs := subject.GetPairs()
		if len(pairs) == 1 && len(pairs[0].GetName()) > 0 && !unicode.IsDigit(rune(pairs[0].GetName()[0])) {
			p := pairs[0]
			fmt.Fprintf(&sb, "%d: %s | %s | %s", i+1, escapeMD(p.GetName()), escapeMD(p.GetType()), escapeMD(p.GetGroup()))
			sb.WriteString(formatClass(p.GetClass()))
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
			fmt.Fprintf(&sb, "%s | %s | %s", escapeMD(p.GetName()), escapeMD(p.GetType()), escapeMD(p.GetGroup()))
			sb.WriteString(formatClass(p.GetClass()))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
