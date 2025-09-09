package models

type Schedule struct {
	Groups map[int]Group `json:"groups"`
}

type Group struct {
	Week string `json:"week"`
	Days []Day  `json:"days"`
}

type Day struct {
	Name     string    `json:"name"`
	Subjects []Subject `json:"subjects"`
}

type Subject struct {
	Pairs   []Pair `json:"pairs"`
	IsEmpty bool   `json:"empty"`
}

type Pair struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Teacher string `json:"teacher"`
	Class   string `json:"class,omitempty"`
}
