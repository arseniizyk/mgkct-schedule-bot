package delivery

import (
	"fmt"
	"strings"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

func formatScheduleDay(resp *pb.GroupScheduleResponse) string {
	var sb strings.Builder
	day := resp.Day[0]
	sb.WriteString(fmt.Sprintf("%s\n", day.Name))

	for i, subj := range day.Subjects {
		if subj.Empty {
			if i >= 3 {
				return sb.String()
			}
			sb.WriteString(fmt.Sprintf("%d | -- | --\n", i+1))
			continue
		}

		sb.WriteString(fmt.Sprintf("%d | %s | %s\n", i+1, subj.Name, subj.Class))
	}

	sb.WriteString("\n")

	return sb.String()
}

func formatScheduleWeek(resp *pb.GroupScheduleResponse) string {
	var sb strings.Builder

day:
	for _, day := range resp.Day {
		sb.WriteString(fmt.Sprintf("%s\n", day.Name))

		for i, subj := range day.Subjects {
			if subj.Empty {
				if i >= 3 {
					continue day
				}
				sb.WriteString(fmt.Sprintf("%d | -- | --\n", i+1))
				continue
			}

			sb.WriteString(fmt.Sprintf("%d | %s | %s\n", i+1, subj.Name, subj.Class))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
