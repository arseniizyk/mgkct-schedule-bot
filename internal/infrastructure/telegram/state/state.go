package state

import (
	"context"
	"sync"
	"time"
)

type State string

const (
	None                   State = ""
	WaitingGroup           State = "waiting_group"
	WaitingTeacher         State = "waiting_teacher"
	WaitingTeacherSchedule State = "waiting_teacher_schedule"
	WaitingTeacherDay      State = "waiting_teacher_day"
	WaitingCabinet         State = "waiting_cabinet"
	WaitingSubject         State = "waiting_subject"
	WaitingArchive         State = "waiting_archive"
	WaitingBroadcast       State = "waiting_broadcast"
)

const defaultTTL = 15 * time.Minute

type stateEntry struct {
	state     State
	updatedAt time.Time
}

type MemoryManager struct {
	mu     sync.RWMutex
	states map[int64]stateEntry
	ttl    time.Duration
}

func NewMemoryManager() *MemoryManager {
	return NewMemoryManagerWithTTL(defaultTTL)
}

func NewMemoryManagerWithTTL(ttl time.Duration) *MemoryManager {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &MemoryManager{
		states: make(map[int64]stateEntry),
		ttl:    ttl,
	}
}

func (m *MemoryManager) Get(chatID int64) (State, bool) {
	m.mu.RLock()
	entry, ok := m.states[chatID]
	m.mu.RUnlock()

	if !ok || entry.state == None {
		return None, false
	}

	if time.Since(entry.updatedAt) > m.ttl {
		m.mu.Lock()
		delete(m.states, chatID)
		m.mu.Unlock()
		return None, false
	}

	return entry.state, true
}

func (m *MemoryManager) Set(chatID int64, st State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if st == None {
		delete(m.states, chatID)
		return nil
	}

	m.states[chatID] = stateEntry{
		state:     st,
		updatedAt: time.Now(),
	}
	return nil
}

func (m *MemoryManager) Clear(chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, chatID)
	return nil
}

func (m *MemoryManager) Cleanup() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	for chatID, entry := range m.states {
		if now.Sub(entry.updatedAt) > m.ttl || entry.state == None {
			delete(m.states, chatID)
		}
	}
}

func (m *MemoryManager) StartCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Cleanup()
		}
	}
}
