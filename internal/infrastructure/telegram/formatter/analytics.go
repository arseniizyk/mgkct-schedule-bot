package formatter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

var pairEndTimes = map[int]string{
	1: "до 10:00 (перерыв до 10:40)",
	2: "до 12:00 (перерыв до 12:40)",
	3: "до 14:00 (перерыв до 14:40)",
	4: "до 16:30 (перерыв до 16:40)",
	5: "до 18:20 (перерыв до 18:30)",
	6: "до 20:10",
}

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
	fmt.Fprintf(&sb, "🚪 *Кабинет %s*\n*%s*\n\n", escapeMD(cs.Cabinet), escapeMD(day.DayName))

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
		fmt.Fprintf(&sb, "*%s*\n", escapeMD(day.DayName))
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

// FormatEndings форматирует аналитику окончания пар по группам
func FormatEndings(stat *entities.EndingsStat) string {
	if stat == nil {
		return "Данные об окончании пар отсутствуют."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "⏰ *Окончание пар: %s*\n_Всего групп в расписании: %d_\n\n", escapeMD(stat.DayName), stat.Total)

	for pairNum := 1; pairNum <= 6; pairNum++ {
		groups, ok := stat.Pairs[pairNum]
		if !ok || len(groups) == 0 {
			continue
		}

		timeNote := pairEndTimes[pairNum]
		if timeNote != "" {
			fmt.Fprintf(&sb, "🔔 *После %d пары* (%s) — %d гр.:\n", pairNum, timeNote, len(groups))
		} else {
			fmt.Fprintf(&sb, "🔔 *После %d пары* — %d гр.:\n", pairNum, len(groups))
		}

		groupStrs := make([]string, 0, len(groups))
		for _, g := range groups {
			groupStrs = append(groupStrs, strconv.Itoa(int(g)))
		}
		fmt.Fprintf(&sb, "└ %s\n\n", strings.Join(groupStrs, ", "))
	}

	if len(stat.DayOff) > 0 {
		groupStrs := make([]string, 0, len(stat.DayOff))
		for _, g := range stat.DayOff {
			groupStrs = append(groupStrs, strconv.Itoa(int(g)))
		}
		fmt.Fprintf(&sb, "🏖 *Выходной (0 пар)* — %d гр.:\n└ %s\n\n", len(stat.DayOff), strings.Join(groupStrs, ", "))
	}

	return sb.String()
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
