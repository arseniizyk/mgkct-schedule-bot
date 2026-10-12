package entities

import "time"

type FacultyMember struct {
	ID            int64     `json:"id"`
	FullName      string    `json:"full_name"`
	ShortName     string    `json:"short_name"`
	Position      string    `json:"position,omitempty"`
	Qualification string    `json:"qualification,omitempty"`
	Education     string    `json:"education,omitempty"`
	Experience    string    `json:"experience,omitempty"`
	Department    string    `json:"department,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SubjectInfo struct {
	ShortName string `json:"short_name"`
	FullName  string `json:"full_name"`
}
