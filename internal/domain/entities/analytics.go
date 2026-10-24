package entities

import (
	"fmt"
	"strconv"
	"time"
)

type CabinetBooking struct {
	DayIndex    int    `json:"day_index"`
	DayName     string `json:"day_name"`
	PairIndex   int    `json:"pair_index"`
	GroupID     int32  `json:"group_id"`
	Subject     string `json:"subject"`
	SubjectFull string `json:"subject_full,omitempty"`
	Type        string `json:"type,omitempty"`
	Teacher     string `json:"teacher,omitempty"`
	Class       string `json:"class"`
}

type CabinetDaySchedule struct {
	DayIndex int               `json:"day_index"`
	DayName  string            `json:"day_name"`
	Bookings []*CabinetBooking `json:"bookings"`
}

type CabinetSchedule struct {
	Cabinet string                `json:"cabinet"`
	Week    time.Time             `json:"week"`
	Days    []*CabinetDaySchedule `json:"days"`
}

type GroupEnding struct {
	GroupID  int32 `json:"group_id"`
	Subgroup int   `json:"subgroup,omitempty"` // 0 - вся группа, 1 - 1-я п/г, 2 - 2-я п/г
}

func (g GroupEnding) String() string {
	if g.Subgroup > 0 {
		return fmt.Sprintf("%d (%d п/г)", g.GroupID, g.Subgroup)
	}
	return strconv.Itoa(int(g.GroupID))
}

type EndingsStat struct {
	DayIndex int                   `json:"day_index"`
	DayName  string                `json:"day_name"`
	Week     time.Time             `json:"week"`
	Pairs    map[int][]GroupEnding `json:"pairs"`   // пара -> список групп/подгрупп
	DayOff   []GroupEnding         `json:"day_off"` // группы без занятий
	Total    int                   `json:"total"`   // всего групп
}

type SubjectStat struct {
	Subject     string         `json:"subject"`
	SubjectFull string         `json:"subject_full,omitempty"`
	Count       int            `json:"count"`
	Types       map[string]int `json:"types,omitempty"`
}
