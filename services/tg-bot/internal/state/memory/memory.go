package memory

import (
	"sync"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/state"
)

type Memory struct {
	mu       *sync.RWMutex
	stateMap map[int64]state.State
}

func NewMemory() state.Manager {
	return &Memory{
		mu:       &sync.RWMutex{},
		stateMap: make(map[int64]state.State),
	}
}

func (s *Memory) Clear(chatID int64) error {
	s.mu.Lock()
	delete(s.stateMap, chatID)
	s.mu.Unlock()
	return nil
}

func (s *Memory) Get(chatID int64) (state state.State, exists bool) {
	s.mu.RLock()
	state, exists = s.stateMap[chatID]
	s.mu.RUnlock()
	return
}

func (s *Memory) Set(chatID int64, state state.State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateMap[chatID] = state
	return nil
}
