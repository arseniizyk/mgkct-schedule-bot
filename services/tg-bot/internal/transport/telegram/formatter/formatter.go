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

	first := findFirstSubject(day.Subjects)

	skip := 0
	if first >= secondShiftMinIndex {
		fmt.Fprintf(&sb, "*%s %s\n*", escapeMD(day.Name), messages.SecondShift)
		skip = first // чужие слоты первой смены не показываем
	} else {
		fmt.Fprintf(&sb, "*%s\n*", escapeMD(day.Name))
	}

	sb.WriteString(formatSubjects(day.Subjects, skip))
	sb.WriteString("\n")

	return sb.String()
}

func findFirstSubject(subjects []*pb.Subject) int {
	for i, subject := range subjects {
		if !subject.IsEmpty {
			return i
		}
	}

	return -1
}

// EffectiveDayIndex возвращает индекс дня, который показываем по умолчанию:
// сегодня, а если пары закончились или день пуст — следующий рабочий день.
func EffectiveDayIndex(group *pb.Group) int {
	today := (int(time.Now().Weekday()) + 6) % 7 // Пн=0 .. Вс=6

	if today >= len(group.Days) {
		return 0
	}

	lastSubject := findLastSubject(group.Days[today].Subjects)
	if lastSubject == -1 {
		return nextWorkDayIndex(today)
	}

	if endTime, ok := getEndTime(today, lastSubject); ok && !time.Now().Before(endTime) {
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

	if dayIdx >= len(group.Days) {
		return formatScheduleDay(group.Days[0])
	}

	return formatScheduleDay(group.Days[dayIdx])
}

// FormatScheduleDayAt форматирует конкретный день недели (0 — понедельник).
func FormatScheduleDayAt(group *pb.Group, dayIdx int) (string, error) {
	if dayIdx < 0 || dayIdx >= len(group.Days) {
		return "", fmt.Errorf("formatter: day index %d out of range (%d days)", dayIdx, len(group.Days))
	}

	return formatScheduleDay(group.Days[dayIdx]), nil
}

var weekdaysTimeEnd = map[int][2]int{ // map[subjectIndex][hours, min]
	1: {10, 40},
	2: {12, 40},
	3: {14, 40},
	4: {16, 30},
	5: {18, 20},
	6: {20, 10},
}

var weekendTimeEnd = map[int][2]int{ // map[subjectIndex][hours, min]
	1: {10, 40},
	2: {12, 40},
	3: {14, 30},
	4: {16, 20},
	5: {18, 10},
	6: {20, 00},
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
	sb.Grow(len(group.Days) * 128)

	for _, day := range group.Days {
		sb.WriteString(formatScheduleDay(day))
	}

	return sb.String()
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

		if subject.IsEmpty {
			if i > lastSubject {
				break
			}
			fmt.Fprintf(&sb, "%d: ──\n", i+1)
			continue
		}

		pairs := subject.Pairs
		if len(pairs) == 1 && !unicode.IsDigit(rune(pairs[0].Name[0])) { // If only 1 pair in subject and starts with digit
			p := pairs[0]
			fmt.Fprintf(&sb, "%d: %s | %s | %s", i+1, escapeMD(p.Name), escapeMD(p.Type), escapeMD(p.Teacher))
			sb.WriteString(formatClass(p.Class))
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
			fmt.Fprintf(&sb, "%s | %s | %s", escapeMD(p.Name), escapeMD(p.Type), escapeMD(p.Teacher))
			sb.WriteString(formatClass(p.Class))
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

// escapeMD экранирует спецсимволы legacy Markdown в данных с сайта,
// иначе Telegram отклоняет сообщение целиком (400 Bad Request).
var mdEscaper = strings.NewReplacer(
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
		if !subject.IsEmpty {
			return i
		}
	}

	return -1
}
