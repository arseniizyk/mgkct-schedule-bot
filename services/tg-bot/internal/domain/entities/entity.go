package entities

import "time"

type User struct {
	ChatID      int64
	Username    string
	Group       int
	TeacherName string
}

func NewUser(chatID int64, username string) User {
	return User{
		ChatID:   chatID,
		Username: username,
	}
}

type Weeks struct {
	Prev    time.Time
	Current time.Time
	Next    time.Time
}
