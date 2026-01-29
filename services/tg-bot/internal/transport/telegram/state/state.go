package state

import (
	"sync"
)

type State string

var (
	WaitingGroup State = "waiting group"
)

type StateManager struct {
	mu       *sync.RWMutex
	stateMap map[int64]State
}

func NewMemory() *StateManager {
	return &StateManager{
		mu:       &sync.RWMutex{},
		stateMap: make(map[int64]State),
	}
}

func (s *StateManager) Clear(chatID int64) error {
	s.mu.Lock()
	delete(s.stateMap, chatID)
	s.mu.Unlock()
	return nil
}

func (s *StateManager) Get(chatID int64) (state State, exists bool) {
	s.mu.RLock()
	state, exists = s.stateMap[chatID]
	s.mu.RUnlock()
	return
}

func (s *StateManager) Set(chatID int64, state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateMap[chatID] = state
	return nil
}
