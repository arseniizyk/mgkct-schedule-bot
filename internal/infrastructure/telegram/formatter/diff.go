package formatter

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

var russianMonths = map[time.Month]string{
	time.January:   "января",
	time.February:  "февраля",
	time.March:     "марта",
	time.April:     "апреля",
	time.May:       "мая",
	time.June:      "июня",
	time.July:      "июля",
	time.August:    "августа",
	time.September: "сентября",
	time.October:   "октября",
	time.November:  "ноября",
	time.December:  "декабря",
}

// FormatGroupScheduleDiff форматирует изменения в расписании учебной группы в виде HTML-сообщения.
func FormatGroupScheduleDiff(diff *entities.ScheduleDiff) string {
	if diff == nil || len(diff.Days) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("🔔 <b>Изменения в расписании</b>\n")

	for i, day := range diff.Days {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "📅 <b>%s, %d %s</b>\n\n",
			html.EscapeString(day.DayName),
			day.Date.Day(),
			russianMonths[day.Date.Month()],
		)

		for pIdx, pair := range day.Pairs {
			for sgIdx, sg := range pair.SubgroupDiffs {
				if pIdx > 0 || sgIdx > 0 {
					sb.WriteString("\n")
				}

				if sg.Subgroup != "" {
					fmt.Fprintf(&sb, "▫️ <b>%d пара • %s</b>\n", pair.Number, html.EscapeString(sg.Subgroup))
				} else {
					fmt.Fprintf(&sb, "▫️ <b>%d пара</b>\n", pair.Number)
				}

				switch sg.Type {
				case entities.ChangeTypeModified:
					if isOnlyAuditoryChange(sg.OldPair, sg.NewPair) {
						hdr := formatGroupPairWithoutAuditory(sg.OldPair)
						formatAuditoryDiff(&sb, hdr, sg.OldPair.Class, sg.NewPair.Class)
					} else {
						oldStr := formatGroupPairHTML(sg.OldPair)
						newStr := formatGroupPairHTML(sg.NewPair)
						fmt.Fprintf(&sb, "  <s>%s</s>\n  👉 <b>%s</b>\n", oldStr, newStr)
					}
				case entities.ChangeTypeAdded:
					newStr := formatGroupPairHTML(sg.NewPair)
					fmt.Fprintf(&sb, "  🆕 <i>Новая пара:</i> <b>%s</b>\n", newStr)
				case entities.ChangeTypeRemoved:
					oldStr := formatGroupPairHTML(sg.OldPair)
					fmt.Fprintf(&sb, "  ❌ <i>Отменена:</i> <s>%s</s>\n", oldStr)
				}
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// FormatTeacherScheduleDiff форматирует изменения в расписании преподавателя в виде HTML-сообщения.
func FormatTeacherScheduleDiff(diff *entities.ScheduleDiff) string {
	if diff == nil || len(diff.Days) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("🔔 <b>Изменения в расписании</b>\n")

	for i, day := range diff.Days {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "📅 <b>%s, %d %s</b>\n\n",
			html.EscapeString(day.DayName),
			day.Date.Day(),
			russianMonths[day.Date.Month()],
		)

		for pIdx, pair := range day.Pairs {
			for sgIdx, sg := range pair.SubgroupDiffs {
				if pIdx > 0 || sgIdx > 0 {
					sb.WriteString("\n")
				}

				if sg.Subgroup != "" {
					fmt.Fprintf(&sb, "▫️ <b>%d пара • %s</b>\n", pair.Number, html.EscapeString(sg.Subgroup))
				} else {
					fmt.Fprintf(&sb, "▫️ <b>%d пара</b>\n", pair.Number)
				}

				switch sg.Type {
				case entities.ChangeTypeModified:
					if isOnlyAuditoryChange(sg.OldPair, sg.NewPair) {
						hdr := formatTeacherPairWithoutAuditory(sg.OldPair)
						formatAuditoryDiff(&sb, hdr, sg.OldPair.Class, sg.NewPair.Class)
					} else {
						oldStr := formatTeacherPairHTML(sg.OldPair)
						newStr := formatTeacherPairHTML(sg.NewPair)
						fmt.Fprintf(&sb, "  <s>%s</s>\n  👉 <b>%s</b>\n", oldStr, newStr)
					}
				case entities.ChangeTypeAdded:
					newStr := formatTeacherPairHTML(sg.NewPair)
					fmt.Fprintf(&sb, "  🆕 <i>Новая пара:</i> <b>%s</b>\n", newStr)
				case entities.ChangeTypeRemoved:
					oldStr := formatTeacherPairHTML(sg.OldPair)
					fmt.Fprintf(&sb, "  ❌ <i>Отменена:</i> <s>%s</s>\n", oldStr)
				}
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func isOnlyAuditoryChange(oldP, newP *entities.Pair) bool {
	if oldP == nil || newP == nil {
		return false
	}
	return strings.TrimSpace(oldP.Name) == strings.TrimSpace(newP.Name) &&
		strings.TrimSpace(oldP.Type) == strings.TrimSpace(newP.Type) &&
		strings.TrimSpace(oldP.Group) == strings.TrimSpace(newP.Group) &&
		formatAuditory(oldP.Class) != formatAuditory(newP.Class)
}

func formatAuditoryDiff(sb *strings.Builder, subjectHeader, oldClass, newClass string) {
	oldAud := formatAuditory(oldClass)
	newAud := formatAuditory(newClass)

	switch {
	case strings.HasPrefix(oldAud, "ауд. ") && strings.HasPrefix(newAud, "ауд. "):
		oldNum := strings.TrimPrefix(oldAud, "ауд. ")
		newNum := strings.TrimPrefix(newAud, "ауд. ")
		fmt.Fprintf(sb, "  🔄 %s: ауд. <s>%s</s> ➔ <b>%s</b>\n", subjectHeader, html.EscapeString(oldNum), html.EscapeString(newNum))
	case oldAud != "" && newAud != "":
		fmt.Fprintf(sb, "  🔄 %s: <s>%s</s> ➔ <b>%s</b>\n", subjectHeader, html.EscapeString(oldAud), html.EscapeString(newAud))
	case newAud != "":
		fmt.Fprintf(sb, "  🔄 %s: <b>%s</b>\n", subjectHeader, html.EscapeString(newAud))
	default:
		fmt.Fprintf(sb, "  🔄 %s\n", subjectHeader)
	}
}

func formatGroupPairWithoutAuditory(p *entities.Pair) string {
	if p == nil {
		return ""
	}
	var parts []string
	if name := strings.TrimSpace(p.Name); name != "" && name != "-" && name != "—" && name != "–" {
		parts = append(parts, html.EscapeString(name))
	}
	if pType := strings.TrimSpace(p.Type); pType != "" && pType != "-" {
		parts = append(parts, html.EscapeString(pType))
	}
	return strings.Join(parts, " • ")
}

func formatTeacherPairWithoutAuditory(p *entities.Pair) string {
	if p == nil {
		return ""
	}
	var parts []string
	if grp := strings.TrimSpace(p.Group); grp != "" && grp != "-" {
		parts = append(parts, html.EscapeString(grp))
	}
	if name := strings.TrimSpace(p.Name); name != "" && name != "-" && name != "—" && name != "–" {
		parts = append(parts, html.EscapeString(name))
	}
	if pType := strings.TrimSpace(p.Type); pType != "" && pType != "-" {
		parts = append(parts, html.EscapeString(pType))
	}
	return strings.Join(parts, " • ")
}

func formatGroupPairHTML(p *entities.Pair) string {
	if p == nil {
		return ""
	}
	var parts []string
	if name := strings.TrimSpace(p.Name); name != "" && name != "-" && name != "—" && name != "–" {
		parts = append(parts, html.EscapeString(name))
	}
	if pType := strings.TrimSpace(p.Type); pType != "" && pType != "-" {
		parts = append(parts, html.EscapeString(pType))
	}
	if aud := formatAuditory(p.Class); aud != "" {
		parts = append(parts, html.EscapeString(aud))
	}
	return strings.Join(parts, " • ")
}

func formatTeacherPairHTML(p *entities.Pair) string {
	if p == nil {
		return ""
	}
	var parts []string
	if grp := strings.TrimSpace(p.Group); grp != "" && grp != "-" {
		parts = append(parts, html.EscapeString(grp))
	}
	if name := strings.TrimSpace(p.Name); name != "" && name != "-" && name != "—" && name != "–" {
		parts = append(parts, html.EscapeString(name))
	}
	if pType := strings.TrimSpace(p.Type); pType != "" && pType != "-" {
		parts = append(parts, html.EscapeString(pType))
	}
	if aud := formatAuditory(p.Class); aud != "" {
		parts = append(parts, html.EscapeString(aud))
	}
	return strings.Join(parts, " • ")
}

func formatAuditory(class string) string {
	class = strings.TrimSpace(class)
	if class == "" || class == "-" || class == "—" || class == "–" {
		return ""
	}
	lower := strings.ToLower(class)
	if strings.HasPrefix(lower, "ауд") {
		return class
	}
	return "ауд. " + class
}
