package state

type Manager interface {
	Set(chatID int64, state State) error
	Get(chatID int64) (State, bool)
	Clear(chatID int64) error
}

type State string

var (
	WaitingGroup State = "waiting group"
)
