package state

import "sync"

type State string

const (
	None           State = ""
	WaitingGroup   State = "waiting_group"
	WaitingTeacher State = "waiting_teacher"
)

// MemoryManager хранит временные интерактивные состояния пользователей в памяти.
// Это избавляет от лишних дисковых операций в БД при каждой отправленной команде.
type MemoryManager struct {
	states sync.Map // map[int64]State
}

func NewMemoryManager() *MemoryManager {
	return &MemoryManager{}
}

func (m *MemoryManager) Get(chatID int64) (State, bool) {
	val, ok := m.states.Load(chatID)
	if !ok {
		return None, false
	}
	st, ok := val.(State)
	if !ok || st == None {
		return None, false
	}
	return st, true
}

func (m *MemoryManager) Set(chatID int64, st State) error {
	if st == None {
		m.states.Delete(chatID)
		return nil
	}
	m.states.Store(chatID, st)
	return nil
}

func (m *MemoryManager) Clear(chatID int64) error {
	m.states.Delete(chatID)
	return nil
}
