package crawler

type Schedule struct {
	Groups []Group `json:"groups"`
}

type Group struct {
	Number int    `json:"number"`
	Week   string `json:"week"`
	Days   []Day  `json:"days"`
}

type Day struct {
	Name     string    `json:"name"`
	Subjects []Subject `json:"subjects"`
}

type Subject struct {
	Name    string `json:"name"`
	Class   string `json:"class"`
	IsEmpty bool   `json:"empty"`
}
