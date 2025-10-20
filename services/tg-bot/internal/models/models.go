package models

import (
	"time"
)

type User struct {
	ChatID   int64
	Username string
	Group    int
}

type Weeks struct {
	Prev    time.Time
	Current time.Time
	Next    time.Time
}
