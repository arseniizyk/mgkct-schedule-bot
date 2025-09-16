package server

import (
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
)

func daysToProto(days []models.Day) []*pb.Day {
	res := make([]*pb.Day, len(days))

	for i, d := range days {
		pbDay := &pb.Day{
			Name:     d.Name,
			Subjects: subjectsToProto(d.Subjects),
		}
		res[i] = pbDay
	}

	return res
}

func subjectsToProto(subjects []models.Subject) []*pb.Subject {
	res := make([]*pb.Subject, len(subjects))

	for i, s := range subjects {
		if s.IsEmpty {
			res[i] = &pb.Subject{Empty: true}
			continue
		}

		pairs := make([]*pb.Pair, len(s.Pairs))
		for j, p := range s.Pairs {
			pairs[j] = &pb.Pair{
				Name:    p.Name,
				Type:    p.Type,
				Class:   p.Class,
				Teacher: p.Teacher,
			}
		}

		res[i] = &pb.Subject{
			Pairs: pairs,
			Empty: false,
		}
	}

	return res
}
