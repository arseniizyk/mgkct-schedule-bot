package entities

import "time"

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

type EndingsStat struct {
	DayIndex int             `json:"day_index"`
	DayName  string          `json:"day_name"`
	Pairs    map[int][]int32 `json:"pairs"`   // пара (1..6) -> список ID групп
	DayOff   []int32         `json:"day_off"` // группы без занятий
	Total    int             `json:"total"`   // всего групп
}

type SubjectStat struct {
	Subject     string         `json:"subject"`
	SubjectFull string         `json:"subject_full,omitempty"`
	Count       int            `json:"count"`
	Types       map[string]int `json:"types,omitempty"`
}
