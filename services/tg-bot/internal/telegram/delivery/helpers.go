package delivery

import (
	"strconv"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	tele "gopkg.in/telebot.v4"
)

func findLastSubject(subjects []*pb.Subject) int {
	var lastSubject int
	for i := len(subjects) - 1; i >= 0; i-- {
		if !subjects[i].Empty {
			lastSubject = i
			return lastSubject
		}
	}

	return -1
}

func Day(add ...int) int {
	day := int(time.Now().Weekday()+6) % 7
	if add != nil {
		day += add[0]
	}

	if day >= 6 {
		day = 0
	}

	return day
}

func inputNum(c tele.Context) (int, bool) {
	if len(c.Args()) == 0 {
		return 0, false
	}

	num, err := strconv.Atoi(c.Args()[0])
	if err != nil {
		return 0, false
	}

	return num, true
}
