package state

import (
	"testing"
	"time"
)

func TestMemoryManager(t *testing.T) {
	sm := NewMemoryManager()

	st, ok := sm.Get(12345)
	if ok || st != None {
		t.Errorf("expected no state initially, got %v", st)
	}

	if err := sm.Set(12345, WaitingGroup); err != nil {
		t.Fatalf("unexpected error setting state: %v", err)
	}

	st, ok = sm.Get(12345)
	if !ok || st != WaitingGroup {
		t.Errorf("expected WaitingGroup, got %v", st)
	}

	if err := sm.Clear(12345); err != nil {
		t.Fatalf("unexpected error clearing state: %v", err)
	}

	st, ok = sm.Get(12345)
	if ok || st != None {
		t.Errorf("expected no state after clear, got %v", st)
	}
}

func TestMemoryManager_TTLEviction(t *testing.T) {
	// Создаем менеджер с очень маленьким TTL (10ms)
	sm := NewMemoryManagerWithTTL(10 * time.Millisecond)

	_ = sm.Set(1001, WaitingGroup)
	st, ok := sm.Get(1001)
	if !ok || st != WaitingGroup {
		t.Fatalf("expected state to be set, got %v (ok=%v)", st, ok)
	}

	time.Sleep(20 * time.Millisecond)

	// Должен истечь по TTL при Get
	st, ok = sm.Get(1001)
	if ok || st != None {
		t.Errorf("expected state to expire by TTL, got %v", st)
	}

	// Проверяем явный Cleanup
	_ = sm.Set(1002, WaitingTeacher)
	time.Sleep(20 * time.Millisecond)
	sm.Cleanup()

	sm.mu.RLock()
	_, exists := sm.states[int64(1002)]
	sm.mu.RUnlock()
	if exists {
		t.Errorf("expected state 1002 to be removed by Cleanup()")
	}
}
