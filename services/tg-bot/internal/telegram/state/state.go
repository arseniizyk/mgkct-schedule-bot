package state

import (
	"sync"
)

type Manager interface {
	Set(chatID int64, state State) error
	Get(chatID int64) (State, bool)
	Clear(chatID int64) error
}

type State string

var (
	WaitingGroup State = "waiting group"
)

type memory struct {
	mu       *sync.RWMutex
	stateMap map[int64]State
}

func NewMemory() Manager {
	return &memory{
		mu:       &sync.RWMutex{},
		stateMap: make(map[int64]State),
	}
}

func (s *memory) Clear(chatID int64) error {
	s.mu.Lock()
	delete(s.stateMap, chatID)
	s.mu.Unlock()
	return nil
}

func (s *memory) Get(chatID int64) (state State, exists bool) {
	s.mu.RLock()
	state, exists = s.stateMap[chatID]
	s.mu.RUnlock()
	return
}

func (s *memory) Set(chatID int64, state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateMap[chatID] = state
	return nil
}
