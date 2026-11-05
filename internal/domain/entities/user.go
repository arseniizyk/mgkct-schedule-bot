package entities

import "time"

type User struct {
	ChatID         int64      `json:"chat_id"`
	Username       string     `json:"username"`
	Group          int        `json:"group"`
	Subgroup       int        `json:"subgroup"`
	TeacherName    string     `json:"teacher_name"`
	NotifyEvening  bool       `json:"notify_evening"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
}

type ActivityStats struct {
	DAU int `json:"dau"` // Активных за 24 часа
	WAU int `json:"wau"` // Активных за 7 дней
	MAU int `json:"mau"` // Активных за 30 дней
}

type GroupSubgroupTarget struct {
	GroupID  int `json:"group_id"`
	Subgroup int `json:"subgroup"`
}

func NewUser(chatID int64, username string) User {
	return User{
		ChatID:        chatID,
		Username:      username,
		NotifyEvening: true,
	}
}
