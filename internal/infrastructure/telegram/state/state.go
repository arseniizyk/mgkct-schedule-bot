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

// MemoryManager хранит временные интерактивные состояния пользователей в памяти с TTL-эвикшном.
// Это избавляет от лишних дисковых операций в БД при каждой отправленной команде.
type MemoryManager struct {
	states sync.Map // map[int64]stateEntry
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
		ttl: ttl,
	}
}

func (m *MemoryManager) Get(chatID int64) (State, bool) {
	val, ok := m.states.Load(chatID)
	if !ok {
		return None, false
	}
	entry, ok := val.(stateEntry)
	if !ok || entry.state == None {
		m.states.Delete(chatID)
		return None, false
	}
	if time.Since(entry.updatedAt) > m.ttl {
		m.states.Delete(chatID)
		return None, false
	}
	return entry.state, true
}

func (m *MemoryManager) Set(chatID int64, st State) error {
	if st == None {
		m.states.Delete(chatID)
		return nil
	}
	m.states.Store(chatID, stateEntry{
		state:     st,
		updatedAt: time.Now(),
	})
	return nil
}

func (m *MemoryManager) Clear(chatID int64) error {
	m.states.Delete(chatID)
	return nil
}

// Cleanup удаляет все устаревшие состояния из памяти.
func (m *MemoryManager) Cleanup() {
	now := time.Now()
	m.states.Range(func(key, value any) bool {
		if entry, ok := value.(stateEntry); ok {
			if now.Sub(entry.updatedAt) > m.ttl {
				m.states.Delete(key)
			}
		} else {
			m.states.Delete(key)
		}
		return true
	})
}

// StartCleanup запускает периодическую очистку устаревших состояний.
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
