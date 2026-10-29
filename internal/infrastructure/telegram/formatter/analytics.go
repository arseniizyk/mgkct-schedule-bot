package formatter

import (
	"fmt"
	"strings"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

// FormatCabinetDay форматирует расписание занятости кабинета на один день
func FormatCabinetDay(cs *entities.CabinetSchedule, dayIdx int) string {
	if cs == nil || len(cs.Days) == 0 {
		return "Расписание для данного кабинета не найдено."
	}
	if dayIdx < 0 || dayIdx >= len(cs.Days) {
		dayIdx = 0
	}

	day := cs.Days[dayIdx]
	var sb strings.Builder
	tag := DayRelativeLabel(cs.Week, day.DayIndex)
	fmt.Fprintf(&sb, "🚪 *Кабинет %s*\n*%s%s*\n\n", escapeMD(cs.Cabinet), escapeMD(day.DayName), tag)

	if len(day.Bookings) == 0 {
		sb.WriteString("🎉 *Свободен на весь день!*\nПар в этом кабинете не запланировано.\n")
		return sb.String()
	}

	for _, b := range day.Bookings {
		typeStr := ""
		if b.Type != "" {
			typeStr = fmt.Sprintf(" | %s", escapeMD(b.Type))
		}
		teacherStr := ""
		if b.Teacher != "" {
			teacherStr = fmt.Sprintf(" | %s", escapeMD(b.Teacher))
		}
		fmt.Fprintf(&sb, "%d пара: гр. *%d* | %s%s%s\n", b.PairIndex, b.GroupID, escapeMD(b.Subject), typeStr, teacherStr)
	}

	return sb.String()
}

// FormatCabinetWeek форматирует расписание занятости кабинета на всю неделю
func FormatCabinetWeek(cs *entities.CabinetSchedule) string {
	if cs == nil || len(cs.Days) == 0 {
		return "Расписание для данного кабинета не найдено."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🚪 *Расписание кабинета %s на неделю*\n\n", escapeMD(cs.Cabinet))

	for _, day := range cs.Days {
		tag := DayRelativeLabel(cs.Week, day.DayIndex)
		fmt.Fprintf(&sb, "*%s%s*\n", escapeMD(day.DayName), tag)
		if len(day.Bookings) == 0 {
			sb.WriteString("└─ _Свободен_\n\n")
			continue
		}
		for _, b := range day.Bookings {
			teacherStr := ""
			if b.Teacher != "" {
				teacherStr = fmt.Sprintf(" (%s)", escapeMD(b.Teacher))
			}
			fmt.Fprintf(&sb, "└─ %d пара: гр. *%d* — %s%s\n", b.PairIndex, b.GroupID, escapeMD(b.Subject), teacherStr)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatEndings форматирует аналитику окончания пар по группам с разбивкой по корпусам
func FormatEndings(stat *entities.EndingsStat) string {
	if stat == nil {
		return "Данные об окончании пар отсутствуют."
	}

	var sb strings.Builder
	dayTitle := escapeMD(stat.DayName)
	fmt.Fprintf(&sb, "⏰ *Окончание пар: %s*\n\n", dayTitle)

	maxPairNum := 6
	for k := range stat.Pairs {
		if k > maxPairNum {
			maxPairNum = k
		}
	}

	hasCampuses := stat.KazintsaTotalStudents > 0 || stat.KnorinaTotalStudents > 0
	if !hasCampuses {
		for _, groups := range stat.Pairs {
			for _, g := range groups {
				if g.Campus != "" {
					hasCampuses = true
					break
				}
			}
			if hasCampuses {
				break
			}
		}
	}

	if hasCampuses {
		renderCampusEndings(&sb, stat, entities.CampusKazintsa, "🏛", "Казинца", stat.KazintsaStudentsByPair, maxPairNum)
		renderCampusEndings(&sb, stat, entities.CampusKnorina, "🌲", "Кнорина", stat.KnorinaStudentsByPair, maxPairNum)
	} else {
		for pairNum := 1; pairNum <= maxPairNum; pairNum++ {
			groups := stat.Pairs[pairNum]
			if len(groups) == 0 {
				continue
			}
			formatEndingPair(&sb, pairNum, len(groups), stat.StudentsByPair[pairNum], groups)
		}
	}

	if len(stat.DayOff) > 0 {
		groupStrs := make([]string, 0, len(stat.DayOff))
		for _, g := range stat.DayOff {
			groupStrs = append(groupStrs, escapeMD(g.String()))
		}
		if stat.DayOffStudents > 0 {
			fmt.Fprintf(&sb, "🏖 *Выходной (0 пар)* — %d гр. (%d чел.):\n└ %s\n\n", len(stat.DayOff), stat.DayOffStudents, strings.Join(groupStrs, ", "))
		} else {
			fmt.Fprintf(&sb, "🏖 *Выходной (0 пар)* — %d гр.:\n└ %s\n\n", len(stat.DayOff), strings.Join(groupStrs, ", "))
		}
	}

	return strings.TrimSpace(sb.String())
}

func renderCampusEndings(sb *strings.Builder, stat *entities.EndingsStat, campusName, emoji, title string, studentsByPair map[int]int, maxPairNum int) {
	hasPairs := false
	for p := 1; p <= maxPairNum; p++ {
		for _, g := range stat.Pairs[p] {
			camp := g.Campus
			if camp == "" {
				camp = entities.CampusKazintsa
			}
			if camp == campusName {
				hasPairs = true
				break
			}
		}
		if hasPairs {
			break
		}
	}
	if !hasPairs {
		return
	}

	fmt.Fprintf(sb, "%s *Корпус %s:*\n\n", emoji, title)

	for pairNum := 1; pairNum <= maxPairNum; pairNum++ {
		var groups []entities.GroupEnding
		for _, g := range stat.Pairs[pairNum] {
			camp := g.Campus
			if camp == "" {
				camp = entities.CampusKazintsa
			}
			if camp == campusName {
				groups = append(groups, g)
			}
		}
		if len(groups) == 0 {
			continue
		}

		studentsCount := studentsByPair[pairNum]
		formatEndingPair(sb, pairNum, len(groups), studentsCount, groups)
	}
}

func formatEndingPair(sb *strings.Builder, pairNum, groupCount, studentsCount int, groups []entities.GroupEnding) {
	if studentsCount > 0 {
		fmt.Fprintf(sb, "🔔 *После %d пары* — %d гр. (%d чел.):\n", pairNum, groupCount, studentsCount)
	} else {
		fmt.Fprintf(sb, "🔔 *После %d пары* — %d гр.:\n", pairNum, groupCount)
	}

	groupStrs := make([]string, 0, len(groups))
	for _, g := range groups {
		groupStrs = append(groupStrs, escapeMD(g.String()))
	}
	fmt.Fprintf(sb, "└ %s\n\n", strings.Join(groupStrs, ", "))
}

// FormatStats форматирует статистику нагрузки пар
func FormatStats(title string, stats []entities.SubjectStat, totalPairs int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "📊 *Статистика нагрузки: %s*\n_Всего пар на неделю: %d_\n\n", escapeMD(title), totalPairs)

	if len(stats) == 0 {
		sb.WriteString("Занятий не найдено.")
		return sb.String()
	}

	for i, s := range stats {
		percent := 0
		if totalPairs > 0 {
			percent = (s.Count * 100) / totalPairs
		}

		var typesList []string
		for t, cnt := range s.Types {
			typesList = append(typesList, fmt.Sprintf("%s: %d", t, cnt))
		}
		typesStr := ""
		if len(typesList) > 0 {
			typesStr = fmt.Sprintf(" [%s]", strings.Join(typesList, ", "))
		}

		fullStr := ""
		if s.SubjectFull != "" && s.SubjectFull != s.Subject {
			fullStr = fmt.Sprintf("\n   ↳ _%s_", escapeMD(s.SubjectFull))
		}

		fmt.Fprintf(&sb, "%d. *%s* — %d пар (%d%%)%s%s\n", i+1, escapeMD(s.Subject), s.Count, percent, escapeMD(typesStr), fullStr)
	}

	return sb.String()
}
