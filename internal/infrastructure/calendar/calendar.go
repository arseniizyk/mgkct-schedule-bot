package calendar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
)

var minskLocation = time.FixedZone("Europe/Minsk", 3*3600)

// PairTimeSlots определяет время начала и окончания каждой пары (1-based, индексы 0..5 соответствуют 1..6 парам).
// Каждая пара представлена [2][2]int: {{startHour, startMin}, {endHour, endMin}}.
var PairTimeSlots = [6][2][2]int{
	{{9, 0}, {10, 40}},
	{{10, 50}, {12, 40}},
	{{13, 0}, {14, 40}},
	{{14, 50}, {16, 30}},
	{{16, 40}, {18, 20}},
	{{18, 30}, {20, 10}},
}

// SubjectDictionary интерфейс для опционального разрешения полных названий предметов.
type SubjectDictionary interface {
	Resolve(raw string) (cleaned string, fullName string, found bool)
}

// DayOfWeekIndex возвращает индекс дня (0=Пн..6=Вс) по его русскому названию, либо defaultIdx при несовпадении.
func DayOfWeekIndex(dayName string, defaultIdx int) int {
	clean := strings.ToLower(strings.TrimSpace(dayName))
	switch {
	case strings.HasPrefix(clean, "понедельник"):
		return 0
	case strings.HasPrefix(clean, "вторник"):
		return 1
	case strings.HasPrefix(clean, "среда"):
		return 2
	case strings.HasPrefix(clean, "четверг"):
		return 3
	case strings.HasPrefix(clean, "пятница"):
		return 4
	case strings.HasPrefix(clean, "суббота"):
		return 5
	case strings.HasPrefix(clean, "воскресенье"):
		return 6
	default:
		return defaultIdx
	}
}

// GenerateGroupCalendar формирует файл iCalendar (.ics) для учебной группы за указанную неделю.
func GenerateGroupCalendar(group *entities.Group, subgroup int, dict SubjectDictionary) ([]byte, error) {
	if group == nil {
		return nil, fmt.Errorf("calendar: group is nil")
	}

	filtered := formatter.FilterGroupForSubgroup(group, subgroup)
	calName := fmt.Sprintf("Группа %d", group.ID)
	if subgroup > 0 {
		calName += fmt.Sprintf(" (%d п/г)", subgroup)
	}

	var events []event
	weekMonday := getWeekMonday(group.Week)

	for dIdx, day := range filtered.Days {
		if day == nil {
			continue
		}
		actualDayIdx := DayOfWeekIndex(day.Name, dIdx)
		dayDate := weekMonday.AddDate(0, 0, actualDayIdx)

		for subjIdx, subject := range day.Subjects {
			if subject == nil || subject.IsEmpty || subjIdx >= len(PairTimeSlots) {
				continue
			}

			slot := PairTimeSlots[subjIdx]
			start := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), slot[0][0], slot[0][1], 0, 0, minskLocation)
			end := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), slot[1][0], slot[1][1], 0, 0, minskLocation)

			for pairIdx, p := range subject.Pairs {
				if p == nil || (p.Name == "" && p.Class == "") {
					continue
				}

				summary := p.Name
				if dict != nil {
					if _, resolved, ok := dict.Resolve(p.Name); ok && resolved != "" {
						summary = resolved
					}
				}
				if p.Type != "" {
					summary += fmt.Sprintf(" (%s)", p.Type)
				}

				var descParts []string
				if p.Group != "" {
					descParts = append(descParts, fmt.Sprintf("Подгруппа/Инфо: %s", p.Group))
				}
				descParts = append(descParts, fmt.Sprintf("Группа: %d", group.ID))
				descParts = append(descParts, fmt.Sprintf("Пара: %d", subjIdx+1))

				uid := fmt.Sprintf("pair-%s-p%d-sub%d-g%d-%d@mgkct-bot",
					dayDate.Format("20060102"), subjIdx+1, subgroup, group.ID, pairIdx)

				events = append(events, event{
					UID:         uid,
					Start:       start,
					End:         end,
					Summary:     summary,
					Location:    formatLocation(p.Class),
					Description: strings.Join(descParts, "\n"),
				})
			}
		}
	}

	return renderICS(calName, events), nil
}

// GenerateTeacherCalendar формирует файл iCalendar (.ics) для преподавателя за указанную неделю.
func GenerateTeacherCalendar(teacher *entities.Teacher, dict SubjectDictionary) ([]byte, error) {
	if teacher == nil {
		return nil, fmt.Errorf("calendar: teacher is nil")
	}

	calName := fmt.Sprintf("Преподаватель %s", teacher.Name)
	var events []event
	weekMonday := getWeekMonday(teacher.Week)

	for dIdx, day := range teacher.Days {
		if day == nil {
			continue
		}
		actualDayIdx := DayOfWeekIndex(day.Name, dIdx)
		dayDate := weekMonday.AddDate(0, 0, actualDayIdx)

		for subjIdx, subject := range day.Subjects {
			if subject == nil || subject.IsEmpty || subjIdx >= len(PairTimeSlots) {
				continue
			}

			slot := PairTimeSlots[subjIdx]
			start := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), slot[0][0], slot[0][1], 0, 0, minskLocation)
			end := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), slot[1][0], slot[1][1], 0, 0, minskLocation)

			for pairIdx, p := range subject.Pairs {
				if p == nil || (p.Name == "" && p.Group == "") {
					continue
				}

				summary := p.Name
				if dict != nil {
					if _, resolved, ok := dict.Resolve(p.Name); ok && resolved != "" {
						summary = resolved
					}
				}
				if p.Type != "" {
					summary += fmt.Sprintf(" (%s)", p.Type)
				}

				var descParts []string
				if p.Group != "" {
					descParts = append(descParts, fmt.Sprintf("Группа: %s", p.Group))
				}
				descParts = append(descParts, fmt.Sprintf("Преподаватель: %s", teacher.Name))
				descParts = append(descParts, fmt.Sprintf("Пара: %d", subjIdx+1))

				uid := fmt.Sprintf("teacher-%s-p%d-t%s-%d@mgkct-bot",
					dayDate.Format("20060102"), subjIdx+1, sanitizeUID(teacher.Name), pairIdx)

				events = append(events, event{
					UID:         uid,
					Start:       start,
					End:         end,
					Summary:     summary,
					Location:    formatLocation(p.Class),
					Description: strings.Join(descParts, "\n"),
				})
			}
		}
	}

	return renderICS(calName, events), nil
}

type event struct {
	UID         string
	Start       time.Time
	End         time.Time
	Summary     string
	Location    string
	Description string
}

func getWeekMonday(t time.Time) time.Time {
	if t.IsZero() {
		t = time.Now()
	}
	m := t.In(minskLocation)
	weekday := (int(m.Weekday()) + 6) % 7 // Пн=0..Вс=6
	return time.Date(m.Year(), m.Month(), m.Day(), 0, 0, 0, 0, minskLocation).AddDate(0, 0, -weekday)
}

func formatLocation(class string) string {
	class = strings.TrimSpace(class)
	if class == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(class), "ауд") {
		return class
	}
	return "ауд. " + class
}

func sanitizeUID(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	h := sha256.Sum256([]byte(s))
	hashSuffix := hex.EncodeToString(h[:8])
	if sb.Len() == 0 {
		return "person-" + hashSuffix
	}
	return sb.String() + "-" + hashSuffix
}

// renderICS сериализует события в стандарт RFC 5545.
func renderICS(calName string, events []event) []byte {
	var buf bytes.Buffer
	nowUTC := time.Now().UTC().Format("20060102T150405Z")

	writeLine(&buf, "BEGIN:VCALENDAR")
	writeLine(&buf, "VERSION:2.0")
	writeLine(&buf, "PRODID:-//MGKCT//MGKCT Schedule Bot//RU")
	writeLine(&buf, "CALSCALE:GREGORIAN")
	writeLine(&buf, "METHOD:PUBLISH")
	writeLine(&buf, fmt.Sprintf("X-WR-CALNAME:%s", escapeText(calName)))
	writeLine(&buf, "X-WR-TIMEZONE:Europe/Minsk")

	// Блок описания таймзоны Europe/Minsk (UTC+3, без DST)
	writeLine(&buf, "BEGIN:VTIMEZONE")
	writeLine(&buf, "TZID:Europe/Minsk")
	writeLine(&buf, "X-LIC-LOCATION:Europe/Minsk")
	writeLine(&buf, "BEGIN:STANDARD")
	writeLine(&buf, "TZOFFSETFROM:+0300")
	writeLine(&buf, "TZOFFSETTO:+0300")
	writeLine(&buf, "TZNAME:MSK")
	writeLine(&buf, "DTSTART:19700101T000000")
	writeLine(&buf, "END:STANDARD")
	writeLine(&buf, "END:VTIMEZONE")

	for _, ev := range events {
		writeLine(&buf, "BEGIN:VEVENT")
		writeLine(&buf, fmt.Sprintf("UID:%s", ev.UID))
		writeLine(&buf, fmt.Sprintf("DTSTAMP:%s", nowUTC))
		writeLine(&buf, fmt.Sprintf("DTSTART;TZID=Europe/Minsk:%s", ev.Start.Format("20060102T150405")))
		writeLine(&buf, fmt.Sprintf("DTEND;TZID=Europe/Minsk:%s", ev.End.Format("20060102T150405")))
		writeLine(&buf, fmt.Sprintf("SUMMARY:%s", escapeText(ev.Summary)))
		if ev.Location != "" {
			writeLine(&buf, fmt.Sprintf("LOCATION:%s", escapeText(ev.Location)))
		}
		if ev.Description != "" {
			writeLine(&buf, fmt.Sprintf("DESCRIPTION:%s", escapeText(ev.Description)))
		}
		writeLine(&buf, "STATUS:CONFIRMED")
		writeLine(&buf, "TRANSP:OPAQUE")
		writeLine(&buf, "END:VEVENT")
	}

	writeLine(&buf, "END:VCALENDAR")
	return buf.Bytes()
}

// writeLine добавляет строку с окончанием CRLF и соблюдением folding (переноса строк > 75 октетов RFC 5545).
// Перенос производится строго по границам UTF-8 рун.
func writeLine(buf *bytes.Buffer, line string) {
	const maxOctets = 75
	if len(line) <= maxOctets {
		buf.WriteString(line)
		buf.WriteString("\r\n")
		return
	}

	var currentLine strings.Builder
	currentOctets := 0

	for _, r := range line {
		rLen := utf8.RuneLen(r)

		if currentOctets+rLen > maxOctets && currentLine.Len() > 0 {
			buf.WriteString(currentLine.String())
			buf.WriteString("\r\n")
			currentLine.Reset()
			currentLine.WriteByte(' ') // leading space for folding RFC 5545
			currentOctets = 1
		}

		currentLine.WriteRune(r)
		currentOctets += rLen
	}

	if currentLine.Len() > 0 {
		buf.WriteString(currentLine.String())
		buf.WriteString("\r\n")
	}
}

// UnfoldICS удаляет переносы строк по RFC 5545 для удобства парсинга и тестирования.
func UnfoldICS(ics string) string {
	return strings.ReplaceAll(ics, "\r\n ", "")
}

// escapeText экранирует спецсимволы по спецификации RFC 5545 (Section 3.3.11).
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\r\n", "\\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\n")
	return s
}
