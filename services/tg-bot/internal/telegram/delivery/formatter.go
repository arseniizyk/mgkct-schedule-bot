package delivery

import (
	"fmt"
	"strings"
	"unicode"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

func formatScheduleDay(day *pb.Day) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("*%s\n*", day.Name))
	sb.WriteString(formatSubjects(day.Subjects))
	sb.WriteString("\n")

	return sb.String()
}

func formatScheduleWeek(resp *pb.GroupScheduleResponse) string {
	var sb strings.Builder

	for _, day := range resp.Day {
		sb.WriteString(formatScheduleDay(day))
	}

	return sb.String()
}

func formatSubjects(subjects []*pb.Subject) string {
	var sb strings.Builder

	lastSubject := -1
	for i, s := range subjects {
		if !s.Empty {
			lastSubject = i
		}
	}

	for i, subject := range subjects {
		if subject.Empty {
			if i > lastSubject {
				continue
			}
			sb.WriteString(fmt.Sprintf("%d: --\n", i+1))
		}

		if len(subject.Pairs) == 1 && !unicode.IsDigit(rune(subject.Pairs[0].Name[0])) {
			pair := subject.Pairs[0]
			sb.WriteString(fmt.Sprintf("%d: %s | %s | %s", i+1, pair.Name, pair.Type, pair.Teacher))
			if pair.Class != "-" {
				sb.WriteString(fmt.Sprintf(" | %s", pair.Class))
			}
			sb.WriteString("\n")

		} else {
			sb.WriteString(fmt.Sprintf("%d:\n", i+1))
			for j, pair := range subject.Pairs {
				if j == len(subject.Pairs)-1 {
					sb.WriteString("└── ")
				} else {
					sb.WriteString("├── ")
				}
				sb.WriteString(fmt.Sprintf("%s | %s | %s", pair.Name, pair.Type, pair.Teacher))
				if pair.Class != "-" {
					sb.WriteString(fmt.Sprintf(" | %s", pair.Class))
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}
