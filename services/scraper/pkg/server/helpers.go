package server

import (
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
)

func fillDays(days []crawler.Day) []*pb.Day {
	res := make([]*pb.Day, len(days))

	for i, d := range days {
		pbDay := &pb.Day{
			Name:     d.Name,
			Subjects: fillSubjects(d.Subjects),
		}
		res[i] = pbDay
	}

	return res
}

func fillSubjects(subjects []crawler.Subject) []*pb.Subject {
	res := make([]*pb.Subject, len(subjects))

	for i, s := range subjects {
		res[i] = &pb.Subject{
			Name:  s.Name,
			Class: s.Class,
			Empty: s.IsEmpty,
		}
	}

	return res
}
