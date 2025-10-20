package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	tele "gopkg.in/telebot.v4"
)

var weekdaysTimeEnd = map[int][2]int{ // map[subjectIndex][hours, min]
	1: {10, 40},
	2: {12, 40},
	3: {14, 40},
	4: {16, 30},
	5: {18, 20},
	6: {20, 10},
}

var weekendTimeEnd = map[int][2]int{ // map[subjectIndex][hours, min]
	1: {10, 40},
	2: {12, 40},
	3: {14, 30},
	4: {16, 20},
	5: {18, 10},
	6: {20, 00},
}

func getEndTime(dayIdx, lastSubject int) (time.Time, bool) {
	var hhmm [2]int
	var ok bool

	if dayIdx == 5 {
		hhmm, ok = weekendTimeEnd[lastSubject]
	} else {
		hhmm, ok = weekdaysTimeEnd[lastSubject]
	}

	if !ok {
		return time.Time{}, false
	}

	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), hhmm[0], hhmm[1], 0, 0, time.Local)

	return end, true
}

func weekDay(add ...int) int {
	weekDay := int(time.Now().Weekday())

	day := int(weekDay+6) % 7

	if len(add) > 0 {
		day += add[0]
	}

	// skip sunday
	if day >= 6 {
		day = 0
	}

	return day
}

func inputNum(c tele.Context) (int, error) {
	if len(c.Args()) == 0 {
		return 0, nil
	}

	num, err := strconv.Atoi(c.Args()[0])
	if err != nil {
		return 0, models.ErrBadInput
	}

	return num, nil
}

func findLastSubject(subjects []*pb.Subject) int {
	if len(subjects) == 0 || subjects == nil {
		return -1
	}

	for i := len(subjects) - 1; i >= 0; i-- {
		if !subjects[i].IsEmpty {
			return i
		}
	}

	return -1
}

func parseCallbackData(data string) string {
	parts := strings.Split(data, "|")
	if len(parts) > 0 {
		return parts[1]
	}

	return ""
}

func parseCallbackWeekNavigation(c *tele.Callback) (int, time.Time, error) {
	parsed := parseCallbackData(c.Data)
	parts := strings.Split(parsed, ":")
	if len(parts) < 2 {
		return 0, time.Time{}, fmt.Errorf("failed splitting data by parts")
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed parsing group_id to int: %w", err)
	}

	date, err := time.Parse("02.01.2006", parts[1])
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed parsing text to date: %w", err)
	}

	return groupID, date, nil
}
