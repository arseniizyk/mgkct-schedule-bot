package entities

type User struct {
	ChatID      int64  `json:"chat_id"`
	Username    string `json:"username"`
	Group       int    `json:"group"`
	TeacherName string `json:"teacher_name"`
	State       string `json:"state"`
}

func NewUser(chatID int64, username string) User {
	return User{
		ChatID:   chatID,
		Username: username,
	}
}
