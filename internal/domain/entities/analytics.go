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

const (
	CampusKazintsa = "Казинца"
	CampusKnorina  = "Кнорина"
)

type GroupEnding struct {
	GroupID  int32  `json:"group_id"`
	Subgroup int    `json:"subgroup,omitempty"` // 0 - вся группа, 1 - 1-я п/г, 2 - 2-я п/г
	Campus   string `json:"campus,omitempty"`   // "Казинца" или "Кнорина"
}

func (g GroupEnding) String() string {
	if g.Subgroup > 0 {
		return fmt.Sprintf("%d (%d п/г)", g.GroupID, g.Subgroup)
	}
	return strconv.Itoa(int(g.GroupID))
}

type EndingsStat struct {
	DayIndex       int                   `json:"day_index"`
	DayName        string                `json:"day_name"`
	Week           time.Time             `json:"week"`
	Pairs          map[int][]GroupEnding `json:"pairs"`                      // пара -> список групп/подгрупп
	DayOff         []GroupEnding         `json:"day_off"`                    // группы без занятий
	Total          int                   `json:"total"`                      // всего групп
	TotalStudents  int                   `json:"total_students,omitempty"`   // всего учащихся в расписании
	StudentsByPair map[int]int           `json:"students_by_pair,omitempty"` // пара -> количество учащихся
	DayOffStudents int                   `json:"day_off_students,omitempty"` // учащихся на выходном

	// Статистика по корпусам
	KazintsaTotalStudents  int         `json:"kazintsa_total_students,omitempty"`
	KnorinaTotalStudents   int         `json:"knorina_total_students,omitempty"`
	KazintsaStudentsByPair map[int]int `json:"kazintsa_students_by_pair,omitempty"`
	KnorinaStudentsByPair  map[int]int `json:"knorina_students_by_pair,omitempty"`
}

type SubjectStat struct {
	Subject     string         `json:"subject"`
	SubjectFull string         `json:"subject_full,omitempty"`
	Count       int            `json:"count"`
	Types       map[string]int `json:"types,omitempty"`
}
