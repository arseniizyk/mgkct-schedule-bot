package delivery

import (
	"fmt"
	"strings"
	"unicode"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

func formatScheduleDay(day *pb.Day) string {
	var sb strings.Builder
	sb.Grow(256)

	sb.WriteString(fmt.Sprintf("*%s\n*", day.Name))
	sb.WriteString(formatSubjects(day.Subjects))
	sb.WriteString("\n")

	return sb.String()
}

func formatScheduleWeek(resp *pb.GroupScheduleResponse) string {
	var sb strings.Builder
	sb.Grow(len(resp.Day) * 128)

	for _, day := range resp.Day {
		sb.WriteString(formatScheduleDay(day))
	}

	return sb.String()
}

func formatSubjects(subjects []*pb.Subject) string {
	var sb strings.Builder
	sb.Grow(len(subjects) * 80)

	lastSubject := -1
	for i := len(subjects) - 1; i >= 0; i-- {
		if !subjects[i].Empty {
			lastSubject = i
			break
		}
	}

	for i, subject := range subjects {
		if subject.Empty {
			if i > lastSubject {
				break
			}
			sb.WriteString(fmt.Sprintf("%d: ──\n", i+1))
			continue
		}

		pairs := subject.Pairs

		if len(pairs) == 1 && !unicode.IsDigit(rune(pairs[0].Name[0])) { // If only 1 pair in subject and starts with digit
			p := pairs[0]
			sb.WriteString(fmt.Sprintf("%d: %s | %s | %s", i+1, p.Name, p.Type, p.Teacher))
			sb.WriteString(formatClass(p.Class))
			sb.WriteString("\n")
			continue
		}

		sb.WriteString(fmt.Sprintf("%d:\n", i+1))
		for j, p := range pairs {
			if j == len(pairs)-1 { // if last pair in subject
				sb.WriteString("└─ ")
			} else {
				sb.WriteString("├─ ")
			}
			sb.WriteString(fmt.Sprintf("%s | %s | %s", p.Name, p.Type, p.Teacher))
			sb.WriteString(formatClass(p.Class))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatClass(class string) string {
	if class != "-" {
		return " | " + class
	}
	return ""
}
