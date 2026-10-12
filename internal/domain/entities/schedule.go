package entities

import "time"

type Schedule struct {
	Groups map[int32]*Group `json:"groups"`
}

type Group struct {
	ID   int32     `json:"id"`
	Week time.Time `json:"week"`
	Days []*Day    `json:"days"`
}

type Day struct {
	Name     string     `json:"name"`
	Subjects []*Subject `json:"subjects"`
}

type Subject struct {
	Pairs   []*Pair `json:"pairs"`
	IsEmpty bool    `json:"isEmpty,omitempty"`
}

type Pair struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Group string `json:"group"`
	Class string `json:"class"`
}

type Teacher struct {
	Name string    `json:"name"`
	Week time.Time `json:"week"`
	Days []*Day    `json:"days"`
}

type WeekNavigation struct {
	Prev    time.Time `json:"prev"`
	Current time.Time `json:"current"`
	Next    time.Time `json:"next"`
}

type UpdatedGroup struct {
	Group         *Group
	IsWeekUpdated bool
}

type UpdatedTeacher struct {
	Teacher       *Teacher
	IsWeekUpdated bool
}
